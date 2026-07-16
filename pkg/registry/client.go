package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	DefaultRegistryURL  = "https://registry-1.docker.io"
	DefaultAuthURL      = "https://auth.docker.io/token"
	DefaultUserAgent    = "poqman/0.1"
	ConnectTimeout      = 30 * time.Second
	BlobDownloadTimeout = 300 * time.Second
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	authTokens map[string]string
	mu         sync.RWMutex
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: ConnectTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}
				for k, v := range via[0].Header {
					if k == "Authorization" {
						req.Header.Set(k, v[0])
					}
				}
				return nil
			},
		},
		baseURL:    DefaultRegistryURL,
		authTokens: make(map[string]string),
	}
}

func (c *Client) doAuthenticated(registry, repo string, req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registry request failed: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()

		wwwAuth := resp.Header.Get("Www-Authenticate")
		if wwwAuth == "" {
			wwwAuth = resp.Header.Get("www-authenticate")
		}

		if wwwAuth == "" {
			return nil, fmt.Errorf("authentication required but no www-authenticate header")
		}

		authReq, err := ParseAuthHeader(wwwAuth)
		if err != nil {
			return nil, fmt.Errorf("parse auth header: %w", err)
		}

		token, err := FetchToken(authReq)
		if err != nil {
			return nil, fmt.Errorf("fetch auth token: %w", err)
		}

		c.mu.Lock()
		c.authTokens[registry] = token
		c.mu.Unlock()

		req.Header.Set("Authorization", "Bearer "+token)
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("registry request with auth failed: %w", err)
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, &RegistryError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	return resp, nil
}

func (c *Client) doWithRetry(req *http.Request, registry, repo string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		resp, err := c.doAuthenticated(registry, repo, req)
		if err == nil {
			return resp, nil
		}
		if re, ok := err.(*RegistryError); ok && re.StatusCode >= 400 && re.StatusCode < 500 {
			return nil, err
		}
		lastErr = err
	}
	return nil, fmt.Errorf("failed after 3 attempts: %w", lastErr)
}

func (c *Client) GetManifest(registry, repo, reference string) ([]byte, string, error) {
	url := fmt.Sprintf("%s/v2/%s/manifests/%s", c.resolveBase(registry), repo, reference)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create manifest request: %w", err)
	}

	req.Header.Set("User-Agent", DefaultUserAgent)
	c.setAuthHeader(registry, req)

	accepts := []string{
		"application/vnd.oci.image.index.v1+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
	}
	req.Header.Set("Accept", strings.Join(accepts, ", "))

	resp, err := c.doWithRetry(req, registry, repo)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read manifest body: %w", err)
	}

	mediaType := resp.Header.Get("Content-Type")
	if mediaType == "" {
		mediaType = resp.Header.Get("content-type")
	}

	return body, mediaType, nil
}

func (c *Client) GetBlob(registry, repo, digest string) (io.ReadCloser, int64, error) {
	url := fmt.Sprintf("%s/v2/%s/blobs/%s", c.resolveBase(registry), repo, digest)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create blob request: %w", err)
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	c.setAuthHeader(registry, req)

	resp, err := c.doWithRetry(req, registry, repo)
	if err != nil {
		return nil, 0, err
	}

	return resp.Body, resp.ContentLength, nil
}

func (c *Client) setAuthHeader(registry string, req *http.Request) {
	c.mu.RLock()
	token, ok := c.authTokens[registry]
	c.mu.RUnlock()
	if ok {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func (c *Client) resolveBase(registry string) string {
	if registry == "docker.io" || registry == "" {
		return DefaultRegistryURL
	}
	return "https://" + registry
}

type RegistryError struct {
	StatusCode int
	Message    string
}

func (e *RegistryError) Error() string {
	return fmt.Sprintf("registry returned HTTP %d: %s", e.StatusCode, strings.TrimSpace(e.Message))
}

func IsNotFound(err error) bool {
	if re, ok := err.(*RegistryError); ok {
		return re.StatusCode == http.StatusNotFound
	}
	return false
}

type basicResponse struct {
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func parseErrorResponse(body []byte) string {
	var resp basicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return string(body)
	}
	if len(resp.Errors) > 0 {
		return resp.Errors[0].Message
	}
	return string(body)
}
