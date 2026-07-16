package kernel

import (
	"fmt"
	"strings"
)

type AlpineResolver struct{}

func NewAlpineResolver() *AlpineResolver {
	return &AlpineResolver{}
}

func (r *AlpineResolver) Name() string { return "alpine" }

func (r *AlpineResolver) Resolve(req *ResolveRequest) (string, string, error) {
	parts := strings.SplitN(req.Version, ":", 2)
	if len(parts) != 2 && !strings.HasPrefix(req.Version, "3.") && !strings.HasPrefix(req.Version, "edge") {
		return "", "", fmt.Errorf(
			"alpine kernel requires release or release:flavor:version format.\n"+
				"Usage: poqman kernel pull alpine:<release>\n"+
				"       poqman kernel pull alpine:<release>:<flavor>:<version>\n"+
				"Example: poqman kernel pull alpine:3.21\n"+
				"         poqman kernel pull alpine:3.21:lts:6.6.52-0-lts\n"+
				"Find packages at: https://pkgs.alpinelinux.org/packages\n"+
				"Common flavors: lts, virt, standard",
		)
	}

	if len(parts) == 1 {
		release := parts[0]
		flavor := "lts"
		resolved, err := ResolveAlpinePackage(release, flavor, req.Arch)
		if err != nil {
			return "", "", fmt.Errorf(
				"alpine kernel auto-resolution failed: %v\n"+
					"Specify full version manually: alpine:<release>:<flavor>:<version>\n"+
					"Example: alpine:3.21:lts:6.6.52-0-lts",
				err,
			)
		}
		parsed, _ := ParseKernelRef(resolved)
		req.Version = parsed.Version
		parts = strings.SplitN(req.Version, ":", 2)
	}

	if len(parts) == 2 && !strings.Contains(parts[1], ":") {
		release := parts[0]
		flavor := parts[1]
		resolved, err := ResolveAlpinePackage(release, flavor, req.Arch)
		if err != nil {
			return "", "", fmt.Errorf(
				"alpine kernel auto-resolution failed: %v\n"+
					"Specify full version manually: alpine:%s:%s:<version>",
				err, release, flavor,
			)
		}
		parsed, _ := ParseKernelRef(resolved)
		req.Version = parsed.Version
		parts = strings.SplitN(req.Version, ":", 2)
	}

	// Parse: alpine:release:flavor:version
	subParts := strings.SplitN(parts[1], ":", 2)
	release := parts[0]
	flavor := "lts"
	version := parts[1]
	if len(subParts) == 2 {
		flavor = subParts[0]
		version = subParts[1]
	}

	mappedArch := mapAlpineArch(req.Arch)
	url := fmt.Sprintf("https://dl-cdn.alpinelinux.org/alpine/v%s/main/%s/linux-%s-%s.apk",
		release, mappedArch, flavor, version)

	return url, "apk", nil
}

func (r *AlpineResolver) FindKernelInDir(dir string) (string, error) {
	return findKernelBinary(dir)
}

func mapAlpineArch(arch string) string {
	switch arch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "armhf", "arm":
		return "armhf"
	case "riscv64":
		return "riscv64"
	default:
		return arch
	}
}

func ExtractApk(apkPath string, outputDir string) error {
	contentsDir := outputDir
	if err := extractTarGz(apkPath, contentsDir); err != nil {
		return fmt.Errorf("extract apk: %w", err)
	}
	return nil
}
