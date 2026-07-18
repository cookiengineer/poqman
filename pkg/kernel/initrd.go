package kernel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cookiengineer/poqman/pkg/storage"
)

func HasNinePModules(paths *storage.Paths, kernelID string) bool {
	modDir := paths.KernelModulesDir(kernelID)
	if _, err := os.Stat(modDir); os.IsNotExist(err) {
		return false
	}

	modCount := 0
	filepath.Walk(modDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".ko") {
			modCount++
		}
		return nil
	})

	return modCount > 0
}

func BuildInitrd(paths *storage.Paths, kernelID string, initBinary []byte, outputPath string) error {
	if len(initBinary) == 0 {
		return fmt.Errorf("init binary is empty")
	}

	if !HasNinePModules(paths, kernelID) {
		return fmt.Errorf("no 9p kernel modules found for kernel %s", kernelID)
	}

	workDir, err := os.MkdirTemp("", "poqman-initrd-")
	if err != nil {
		return fmt.Errorf("create initrd temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	for _, d := range []string{"bin", "lib/modules", "newroot", "oldroot", "proc", "sys", "dev"} {
		os.MkdirAll(filepath.Join(workDir, d), 0o755)
	}

	initPath := filepath.Join(workDir, "init")
	if err := os.WriteFile(initPath, initBinary, 0o755); err != nil {
		return fmt.Errorf("write init binary: %w", err)
	}

	busyboxPath := findBusybox()
	if busyboxPath != "" {
		copyFileData(busyboxPath, filepath.Join(workDir, "bin", "busybox"))
		os.Chmod(filepath.Join(workDir, "bin", "busybox"), 0o755)
		for _, app := range []string{"sh", "ip", "udhcpc", "ifconfig", "route"} {
			os.Symlink("busybox", filepath.Join(workDir, "bin", app))
		}
	}

	modDir := paths.KernelModulesDir(kernelID)
	filepath.Walk(modDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(modDir, path)
		dest := filepath.Join(workDir, "lib", "modules", relPath)
		os.MkdirAll(filepath.Dir(dest), 0o755)
		copyFileData(path, dest)
		return nil
	})

	if err := os.MkdirAll(filepath.Dir(outputPath), storage.DefaultPerms); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	buildCmd := fmt.Sprintf("cd %s && find . | cpio -o -H newc 2>/dev/null | gzip > %s", workDir, outputPath)
	cmd := exec.Command("sh", "-c", buildCmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build initrd: %w\n%s", err, string(out))
	}

	return nil
}

func copyFileData(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	return os.WriteFile(dst, data, 0o644)
}

func findBusybox() string {
	paths := []string{
		"/bin/busybox",
		"/usr/bin/busybox",
		"/sbin/busybox",
		"/usr/sbin/busybox",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	binary, err := exec.LookPath("busybox")
	if err == nil {
		return binary
	}
	return ""
}
