package kernel

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func ResolveDebianPackage(kernelVersion, arch string) (string, error) {
	pkgName := "linux-image-" + kernelVersion
	if !strings.Contains(kernelVersion, "-"+arch) {
		pkgName = "linux-image-" + kernelVersion + "-" + arch
	}

	url := fmt.Sprintf("https://api.ftp-master.debian.org/madison?package=%s&s=bookworm&a=%s&f=json",
		pkgName, mapDebArch(arch))

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("query debian madison API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if err != nil {
		return "", fmt.Errorf("read madison response: %w", err)
	}

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) >= 2 {
			pkgVer := strings.TrimSpace(parts[1])
			if pkgVer != "" {
				fullRef := fmt.Sprintf("debian:%s:%s", kernelVersion, pkgVer)
				return fullRef, nil
			}
		}
	}

	return "", fmt.Errorf("package %s not found for architecture %s", pkgName, arch)
}

func ResolveAlpinePackage(kernelVersion, flavor, arch string) (string, error) {
	mappedArch := mapAlpineArch(arch)
	release := kernelVersion

	if strings.Contains(kernelVersion, ":") {
		parts := strings.SplitN(kernelVersion, ":", 2)
		release = parts[0]
	}

	pkgName := fmt.Sprintf("linux-%s", flavor)
	url := fmt.Sprintf("https://pkgs.alpinelinux.org/package/v%s/main/%s/%s",
		release, mappedArch, pkgName)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("query alpine packages: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 131072))
	if err != nil {
		return "", fmt.Errorf("read alpine response: %w", err)
	}

	html := string(body)

	versionMarker := `<td class="version">`
	idx := strings.Index(html, versionMarker)
	if idx < 0 {
		return "", fmt.Errorf("version not found on alpine package page for %s", pkgName)
	}

	start := idx + len(versionMarker)
	end := strings.Index(html[start:], "</td>")
	if end < 0 {
		return "", fmt.Errorf("version end not found")
	}

	version := strings.TrimSpace(html[start : start+end])

	fullRef := fmt.Sprintf("alpine:%s:%s:%s", release, flavor, version)
	return fullRef, nil
}

func ResolveArchPackage(kernelVersion string) (string, error) {
	url := "https://archive.archlinux.org/packages/l/linux/"

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("query arch archive: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 262144))
	if err != nil {
		return "", fmt.Errorf("read arch response: %w", err)
	}

	html := string(body)
	searchPattern := fmt.Sprintf("linux-%s-", kernelVersion)

	idx := strings.Index(html, searchPattern)
	if idx < 0 {
		return "", fmt.Errorf("package linux-%s not found in arch archive", kernelVersion)
	}

	startTag := strings.LastIndex(html[:idx], "<a href=\"")
	if startTag < 0 {
		return "", fmt.Errorf("link not found for linux-%s", kernelVersion)
	}

	hrefContent := html[startTag+9:]
	hrefEnd := strings.Index(hrefContent, "\"")
	if hrefEnd < 0 {
		return "", fmt.Errorf("link end not found")
	}

	filename := hrefContent[:hrefEnd]
	filename = strings.TrimSuffix(filename, ".pkg.tar.zst")
	filename = strings.TrimSuffix(filename, ".pkg.tar.xz")

	parts := strings.SplitN(filename, "-", 4)
	if len(parts) < 4 {
		return "", fmt.Errorf("unexpected package name format: %s", filename)
	}

	pkgVer := parts[3]

	fullRef := fmt.Sprintf("archlinux:%s:%s", kernelVersion, pkgVer)
	return fullRef, nil
}
