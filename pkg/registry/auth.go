package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

type AuthRequest struct {
	Realm   string
	Service string
	Scope   string
}

var (
	realmRe   = regexp.MustCompile(`realm="([^"]+)"`)
	serviceRe = regexp.MustCompile(`service="([^"]+)"`)
	scopeRe   = regexp.MustCompile(`scope="([^"]+)"`)
)

func ParseAuthHeader(header string) (*AuthRequest, error) {
	auth := &AuthRequest{}

	if m := realmRe.FindStringSubmatch(header); len(m) > 1 {
		auth.Realm = m[1]
	}
	if m := serviceRe.FindStringSubmatch(header); len(m) > 1 {
		auth.Service = m[1]
	}
	if m := scopeRe.FindStringSubmatch(header); len(m) > 1 {
		auth.Scope = m[1]
	}

	if auth.Realm == "" {
		return nil, fmt.Errorf("no realm in www-authenticate header: %q", header)
	}

	return auth, nil
}

func FetchToken(auth *AuthRequest) (string, error) {
	url := auth.Realm
	if !strings.Contains(url, "?") {
		url += "?"
	} else {
		url += "&"
	}
	url += fmt.Sprintf("service=%s", auth.Service)
	if auth.Scope != "" {
		url += fmt.Sprintf("&scope=%s", auth.Scope)
	}

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("token request returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}

	token := tokenResp.Token
	if token == "" {
		token = tokenResp.AccessToken
	}
	if token == "" {
		return "", fmt.Errorf("no token in auth response")
	}

	return token, nil
}
