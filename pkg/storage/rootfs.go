package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func AssembleRootfs(imageID, containerID string, paths *Paths) (string, error) {
	rootfsPath := paths.ContainerRootfsPath(containerID)
	if err := os.MkdirAll(rootfsPath, DefaultPerms); err != nil {
		return "", fmt.Errorf("create rootfs dir: %w", err)
	}

	layersDir := paths.ImageLayersDir(imageID)

	entries, err := os.ReadDir(layersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return rootfsPath, nil
		}
		return "", fmt.Errorf("read layers dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		layerPath := filepath.Join(layersDir, entry.Name())
		if err := copyDir(layerPath, rootfsPath); err != nil {
			return "", fmt.Errorf("apply layer %s: %w", entry.Name(), err)
		}
	}

	return rootfsPath, nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, target)
		}

		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}

func InjectInit(rootfsPath string, initBinary []byte) error {
	initDir := filepath.Join(rootfsPath, "sbin")
	if err := os.MkdirAll(initDir, 0o755); err != nil {
		return fmt.Errorf("create /sbin dir: %w", err)
	}
	initPath := filepath.Join(initDir, "init")
	return os.WriteFile(initPath, initBinary, 0o755)
}

func CopyKernel(srcPath, dstDir string) error {
	if err := os.MkdirAll(dstDir, DefaultPerms); err != nil {
		return fmt.Errorf("create kernel dir: %w", err)
	}
	return copyFileContent(srcPath, filepath.Join(dstDir, "bzImage"))
}

func copyFileContent(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read kernel: %w", err)
	}
	return os.WriteFile(dst, data, 0o755)
}
