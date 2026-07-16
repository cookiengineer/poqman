package dockerfile

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func createDiffLayer(before map[string]int64, rootfs string, destPath string) (int64, error) {
	file, err := os.Create(destPath)
	if err != nil {
		return 0, fmt.Errorf("create layer file: %w", err)
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	var totalSize int64

	err = filepath.Walk(rootfs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(rootfs, path)
		if relPath == "tmp/poqman-exit-code" || relPath == "tmp/poqman-build.sh" {
			return nil
		}

		beforeTime, existed := before[relPath]
		if existed && beforeTime == info.ModTime().UnixNano() {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("tar header for %s: %w", relPath, err)
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header for %s: %w", relPath, err)
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", relPath, err)
		}
		defer f.Close()

		n, err := io.Copy(tw, f)
		if err != nil {
			return fmt.Errorf("write %s to tar: %w", relPath, err)
		}
		totalSize += n

		return nil
	})
	if err != nil {
		return 0, err
	}

	return totalSize, nil
}

func extractLayerFile(tarPath string, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("open layer tar: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		target := filepath.Join(destDir, filepath.Clean(header.Name))

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, os.FileMode(header.Mode))
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0o755)
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			io.Copy(out, tr)
			out.Close()
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0o755)
			os.Symlink(header.Linkname, target)
		}
	}

	return nil
}
