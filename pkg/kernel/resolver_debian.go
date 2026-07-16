package kernel

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type DebianResolver struct{}

func NewDebianResolver() *DebianResolver {
	return &DebianResolver{}
}

func (r *DebianResolver) Name() string { return "debian" }

func (r *DebianResolver) Resolve(req *ResolveRequest) (string, string, error) {
	parts := strings.SplitN(req.Version, ":", 2)
	pkgVersion := ""
	kernelVersion := req.Version
	if len(parts) == 2 {
		kernelVersion = parts[0]
		pkgVersion = parts[1]
	}

	pkgName := "linux-image-" + kernelVersion
	if !strings.Contains(kernelVersion, "-"+req.Arch) {
		pkgName = "linux-image-" + kernelVersion + "-" + req.Arch
	}

	if pkgVersion == "" {
		return "", "", fmt.Errorf(
			"debian kernel requires full package version.\n"+
				"Usage: poqman kernel pull debian:%s:<debian-pkg-version>\n"+
				"Example: poqman kernel pull debian:6.1.0-25-amd64:6.1.106-3\n"+
				"Find the package version at: https://packages.debian.org/search?keywords=%s",
			req.Version, pkgName,
		)
	}

	mappedArch := mapDebArch(req.Arch)
	url := fmt.Sprintf("http://deb.debian.org/debian/pool/main/l/linux/%s_%s_%s.deb",
		pkgName, pkgVersion, mappedArch)

	return url, "deb", nil
}

func (r *DebianResolver) FindKernelInDir(dir string) (string, error) {
	return findKernelBinary(dir)
}

func mapDebArch(arch string) string {
	switch arch {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	case "armhf", "arm":
		return "armhf"
	case "i386", "386":
		return "i386"
	case "riscv64":
		return "riscv64"
	default:
		return arch
	}
}

func ExtractDeb(debPath string, outputDir string) error {
	cmd := exec.Command("ar", "x", debPath, "--output="+outputDir)
	cmd.Dir = outputDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract deb with ar: %w\n%s", err, string(output))
	}

	dataTar := filepath.Join(outputDir, "data.tar.gz")
	if _, err := os.Stat(dataTar); os.IsNotExist(err) {
		dataTarXz := filepath.Join(outputDir, "data.tar.xz")
		if _, err := os.Stat(dataTarXz); err == nil {
			dataTar = dataTarXz
		} else {
			dataTarZst := filepath.Join(outputDir, "data.tar.zst")
			if _, err := os.Stat(dataTarZst); err == nil {
				dataTar = dataTarZst
			} else {
				return fmt.Errorf("no data.tar.* found in extracted deb package")
			}
		}
	}

	extractDir := filepath.Join(outputDir, "contents")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return fmt.Errorf("create extract dir: %w", err)
	}

	if strings.HasSuffix(dataTar, ".gz") {
		return extractTarGz(dataTar, extractDir)
	} else if strings.HasSuffix(dataTar, ".xz") {
		return extractTarXz(dataTar, extractDir)
	}
	return fmt.Errorf("unsupported archive format: %s", dataTar)
}

func extractTarGz(tarPath, destDir string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("open tar.gz: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzReader.Close()

	return extractTar(gzReader, destDir)
}

func extractTarXz(tarPath, destDir string) error {
	cmd := exec.Command("xzcat", tarPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create xz pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start xzcat: %w", err)
	}
	defer cmd.Wait()

	return extractTar(stdout, destDir)
}

func extractTar(reader io.Reader, destDir string) error {
	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		target := filepath.Join(destDir, filepath.Clean(header.Name))
		if strings.HasPrefix(header.Name, "./") {
			target = filepath.Join(destDir, filepath.Clean(header.Name[2:]))
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, os.FileMode(header.Mode))
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0o755)
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", target, err)
			}
			io.Copy(f, tr)
			f.Close()
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0o755)
			os.Symlink(header.Linkname, target)
		}
	}
	return nil
}
