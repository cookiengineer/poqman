package kernel

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cookiengineer/poqman/pkg/storage"
)

type Resolver interface {
	Resolve(req *ResolveRequest) (downloadURL string, archiveFormat string, err error)
	FindKernelInDir(dir string) (kernelPath string, err error)
	Name() string
}

type ResolverRegistry struct {
	resolvers map[string]Resolver
}

func NewResolverRegistry() *ResolverRegistry {
	reg := &ResolverRegistry{
		resolvers: make(map[string]Resolver),
	}
	reg.resolvers["debian"] = NewDebianResolver()
	reg.resolvers["alpine"] = NewAlpineResolver()
	reg.resolvers["archlinux"] = NewArchLinuxResolver()
	reg.resolvers["oci"] = &OCIResolver{}
	reg.resolvers["http"] = &HTTPResolver{}
	reg.resolvers["https"] = &HTTPResolver{}
	return reg
}

func (r *ResolverRegistry) Resolve(req *ResolveRequest) (Resolver, string, string, error) {
	if req.Arch == "" {
		req.Arch = hostArchKernel()
	}

	resolver, ok := r.resolvers[req.Distro]
	if !ok {
		return nil, "", "", fmt.Errorf("unknown kernel distribution %q (supported: debian, alpine, archlinux, oci, http/https URL)", req.Distro)
	}

	downloadURL, archiveFormat, err := resolver.Resolve(req)
	if err != nil {
		return resolver, "", "", err
	}

	return resolver, downloadURL, archiveFormat, nil
}

func hostArchKernel() string {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	case "arm":
		return "armhf"
	case "386":
		return "i386"
	case "riscv64":
		return "riscv64"
	case "ppc64le":
		return "ppc64le"
	default:
		return runtime.GOARCH
	}
}

type OCIResolver struct{}

func (r *OCIResolver) Name() string { return "oci" }

func (r *OCIResolver) Resolve(req *ResolveRequest) (string, string, error) {
	return req.Version, "oci-image", nil
}

func (r *OCIResolver) FindKernelInDir(dir string) (string, error) {
	return findKernelBinary(dir)
}

type HTTPResolver struct{}

func (r *HTTPResolver) Name() string { return "http" }

func (r *HTTPResolver) Resolve(req *ResolveRequest) (string, string, error) {
	url := req.Version
	format := "tar.gz"
	if strings.HasSuffix(url, ".tar.xz") || strings.HasSuffix(url, ".pkg.tar.xz") {
		format = "tar.xz"
	} else if strings.HasSuffix(url, ".tar.zst") {
		format = "tar.zst"
	} else if strings.HasSuffix(url, ".deb") {
		format = "deb"
	} else if strings.HasSuffix(url, ".apk") {
		format = "apk"
	} else if strings.HasSuffix(url, ".rpm") {
		format = "rpm"
	}
	return url, format, nil
}

func (r *HTTPResolver) FindKernelInDir(dir string) (string, error) {
	return findKernelBinary(dir)
}

func DownloadArchive(url string, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d from %s", resp.StatusCode, url)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), storage.DefaultPerms); err != nil {
		return fmt.Errorf("create download dir: %w", err)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create download file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("write download: %w", err)
	}

	return nil
}

func findKernelBinary(dir string) (string, error) {
	searchPaths := []string{
		"boot/vmlinuz",
		"boot/bzImage",
		"boot/Image",
		"vmlinuz",
		"bzImage",
	}

	for _, searchPath := range searchPaths {
		fullPath := filepath.Join(dir, searchPath)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}

	var found string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		name := filepath.Base(path)
		if strings.HasPrefix(name, "vmlinuz") || strings.HasPrefix(name, "bzImage") || strings.HasPrefix(name, "Image") {
			found = path
			return filepath.SkipAll
		}
		return nil
	})

	if found != "" {
		return found, nil
	}

	return "", fmt.Errorf("no kernel binary found in %s", dir)
}

func extractTarGeneric(archivePath string, outputDir string, decompressor string) error {
	cmd := exec.Command(decompressor, "-d", "-c", archivePath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create %s pipe: %w", decompressor, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", decompressor, err)
	}
	defer cmd.Wait()

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	return extractTar(stdout, outputDir)
}
