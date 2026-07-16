package kernel

import (
	"fmt"
	"strings"
)

type ArchLinuxResolver struct{}

func NewArchLinuxResolver() *ArchLinuxResolver {
	return &ArchLinuxResolver{}
}

func (r *ArchLinuxResolver) Name() string { return "archlinux" }

func (r *ArchLinuxResolver) Resolve(req *ResolveRequest) (string, string, error) {
	parts := strings.SplitN(req.Version, ":", 2)
	pkgVersion := ""
	kernelVersion := req.Version
	if len(parts) == 2 {
		kernelVersion = parts[0]
		pkgVersion = parts[1]
	}

	if pkgVersion == "" {
		resolved, err := ResolveArchPackage(kernelVersion)
		if err != nil {
			return "", "", fmt.Errorf(
				"archlinux kernel requires full package version (auto-resolution failed: %v).\n"+
					"Usage: poqman kernel pull archlinux:<kernel-version>:<pkg-version>\n"+
					"Example: poqman kernel pull archlinux:6.10.10:arch1-1\n"+
					"Find packages at: https://archive.archlinux.org/packages/l/linux/\n"+
					"The pkg version is the archlinux packaging suffix (e.g., arch1-1)",
				err,
			)
		}
		parsed, _ := ParseKernelRef(resolved)
		req.Version = parsed.Version
		parts = strings.SplitN(req.Version, ":", 2)
		kernelVersion = parts[0]
		pkgVersion = ""
		if len(parts) == 2 {
			pkgVersion = parts[1]
		}
	}

	if pkgVersion == "" {
		return "", "", fmt.Errorf(
			"archlinux kernel requires full package version.\n"+
				"Usage: poqman kernel pull archlinux:<kernel-version>:<pkg-version>\n"+
				"Example: poqman kernel pull archlinux:6.10.10:arch1-1\n"+
				"Find packages at: https://archive.archlinux.org/packages/l/linux/\n"+
				"The pkg version is the archlinux packaging suffix (e.g., arch1-1)",
		)
	}

	mappedArch := mapArchArch(req.Arch)
	url := fmt.Sprintf("https://archive.archlinux.org/packages/l/linux/linux-%s-%s.pkg.tar.zst",
		kernelVersion, pkgVersion+"-"+mappedArch)

	return url, "tar.zst", nil
}

func (r *ArchLinuxResolver) FindKernelInDir(dir string) (string, error) {
	return findKernelBinary(dir)
}

func mapArchArch(arch string) string {
	switch arch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return arch
	}
}

func ExtractPkgTarZst(zstPath string, outputDir string) error {
	return extractTarGeneric(zstPath, outputDir, "zstdcat")
}
