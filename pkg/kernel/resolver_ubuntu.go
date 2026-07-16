package kernel

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

type UbuntuResolver struct{}

func NewUbuntuResolver() *UbuntuResolver {
	return &UbuntuResolver{}
}

func (r *UbuntuResolver) Name() string { return "ubuntu" }

func (r *UbuntuResolver) Resolve(req *ResolveRequest) (string, string, error) {
	parts := strings.SplitN(req.Version, ":", 2)
	pkgVersion := ""
	kernelVersion := req.Version
	if len(parts) == 2 {
		kernelVersion = parts[0]
		pkgVersion = parts[1]
	}

	pkgName := "linux-image-" + kernelVersion
	if !strings.Contains(kernelVersion, "-") {
		pkgName = "linux-image-" + kernelVersion + "-generic"
	}

	if pkgVersion == "" {
		resolved, err := ResolveUbuntuPackage(kernelVersion, req.Arch)
		if err != nil {
			return "", "", fmt.Errorf(
				"ubuntu kernel requires full package version (auto-resolution failed: %v).\n"+
					"Usage: poqman kernel pull ubuntu:<version>:<pkg-version>\n"+
					"Example: poqman kernel pull ubuntu:7.0.0-28-generic:7.0.0-28.28\n"+
					"Find packages at: http://archive.ubuntu.com/ubuntu/pool/main/l/linux-signed/",
				err,
			)
		}
		parsed, _ := ParseKernelRef(resolved)
		req.Version = parsed.Version
		parts = strings.SplitN(parsed.Version, ":", 2)
		kernelVersion = parts[0]
		pkgVersion = ""
		if len(parts) == 2 {
			pkgVersion = parts[1]
		}
	}

	if pkgVersion == "" {
		return "", "", fmt.Errorf(
			"ubuntu kernel requires full package version.\n"+
				"Usage: poqman kernel pull ubuntu:%s:<pkg-version>\n"+
				"Example: poqman kernel pull ubuntu:7.0.0-28-generic:7.0.0-28.28\n"+
				"Find packages at: http://archive.ubuntu.com/ubuntu/pool/main/l/linux-signed/",
			req.Version,
		)
	}

	mappedArch := mapUbuntuArch(req.Arch)
	poolPath := "linux-signed"
	url := fmt.Sprintf("http://archive.ubuntu.com/ubuntu/pool/main/l/%s/%s_%s_%s.deb",
		poolPath, pkgName, pkgVersion, mappedArch)

	return url, "deb", nil
}

func (r *UbuntuResolver) FindKernelInDir(dir string) (string, error) {
	return findKernelBinary(dir)
}

func mapUbuntuArch(arch string) string {
	switch arch {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	case "armhf", "arm":
		return "armhf"
	case "i386", "386":
		return "i386"
	case "ppc64el":
		return "ppc64el"
	default:
		return arch
	}
}

func ResolveUbuntuPackage(kernelVersion, arch string) (string, error) {
	mappedArch := mapUbuntuArch(arch)

	pkgPrefix := "linux-image-" + kernelVersion + "_"

	url := "http://archive.ubuntu.com/ubuntu/pool/main/l/linux-signed/"
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("query ubuntu pool: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 524288))
	if err != nil {
		return "", fmt.Errorf("read ubuntu pool: %w", err)
	}

	html := string(body)
	idx := strings.Index(html, pkgPrefix)
	if idx < 0 {
		return "", fmt.Errorf("package %s not found in ubuntu pool", pkgPrefix)
	}

	startTag := strings.LastIndex(html[:idx], "<a href=\"")
	if startTag < 0 {
		return "", fmt.Errorf("link not found for %s", pkgPrefix)
	}

	hrefContent := html[startTag+9:]
	hrefEnd := strings.Index(hrefContent, "\"")
	if hrefEnd < 0 {
		return "", fmt.Errorf("link end not found")
	}

	filename := hrefContent[:hrefEnd]
	filename = strings.TrimSuffix(filename, "_"+mappedArch+".deb")

	pkgNameParts := strings.SplitN(filename, "_", 2)
	if len(pkgNameParts) < 2 {
		return "", fmt.Errorf("unexpected package name: %s", filename)
	}

	pkgVer := pkgNameParts[1]
	fullRef := fmt.Sprintf("ubuntu:%s:%s", kernelVersion, pkgVer)
	return fullRef, nil
}
