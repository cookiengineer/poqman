package storage

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ExtractLayer(reader io.Reader, expectedDigest string, destDir string) error {
	digest := strings.TrimPrefix(expectedDigest, "sha256:")
	hash := sha256.New()

	teeReader := io.TeeReader(reader, hash)

	gzReader, err := gzip.NewReader(teeReader)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		target := filepath.Join(destDir, filepath.Clean(header.Name))

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create dir %s: %w", target, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create parent dir for %s: %w", target, err)
			}

			file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", target, err)
			}

			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return fmt.Errorf("write file %s: %w", target, err)
			}
			file.Close()

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create parent dir for symlink %s: %w", target, err)
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("create symlink %s: %w", target, err)
			}

		case tar.TypeLink:
			linkTarget := filepath.Join(destDir, filepath.Clean(header.Linkname))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create parent dir for hardlink %s: %w", target, err)
			}
			if err := os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("create hardlink %s -> %s: %w", target, linkTarget, err)
			}

		case tar.TypeChar, tar.TypeBlock:
			// skip device nodes (requires root)

		case tar.TypeFifo:
			// skip named pipes
		}

		if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
			// chmod can fail on some filesystems; not fatal
		}
	}

	actualDigest := fmt.Sprintf("%x", hash.Sum(nil))
	if digest != "" && actualDigest != digest {
		return fmt.Errorf("digest mismatch: expected %s, got %s", digest[:12], actualDigest[:12])
	}

	return nil
}

