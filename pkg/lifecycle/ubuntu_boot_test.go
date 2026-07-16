package lifecycle

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cookiengineer/poqman/pkg/dockerfile"
	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func TestIntegration_UbuntuLatestLTSWithUbuntuKernel(t *testing.T) {
	if !hasQEMU() {
		t.Skip("QEMU not installed")
	}

	kernelRef := "ubuntu:7.1.0-5-generic"

	contextDir := t.TempDir()
	dockerfileContent := fmt.Sprintf(`FROM ubuntu:latest
KERNEL "%s"
ENV OS_NAME=ubuntu
ENV OS_VERSION=latest-lts
RUN echo "ubuntu-build-test" > /tmp/build-marker
COPY Dockerfile /opt/Dockerfile.bak
CMD ["/bin/bash"]
`, kernelRef)
	os.WriteFile(filepath.Join(contextDir, "Dockerfile"), []byte(dockerfileContent), 0o644)

	tag := "localhost/poqman-test/ubuntu-lts-kernel:latest"
	opts := dockerfile.BuildOptions{Tag: tag, ContextPath: contextDir}

	t.Logf("Building image with %s...", kernelRef)
	img, err := dockerfile.Build(opts)
	if err != nil {
		t.Fatalf("Build with ubuntu kernel failed: %v", err)
	}

	paths, _ := storage.ResolvePaths()
	imgStore := image.NewStore(paths)

	loaded, err := imgStore.Get(img.ID)
	if err != nil {
		t.Fatalf("get built image: %v", err)
	}

	if loaded.KernelRef != kernelRef {
		t.Fatalf("expected KernelRef %q, got %q", kernelRef, loaded.KernelRef)
	}

	kernelPath := paths.ImageKernelPath(img.ID)
	if _, err := os.Stat(kernelPath); os.IsNotExist(err) {
		t.Fatalf("Ubuntu kernel binary not found at %s", kernelPath)
	}

	kernelInfo, _ := os.Stat(kernelPath)
	t.Logf("Ubuntu kernel binary: %s (%d bytes)", filepath.Base(kernelPath), kernelInfo.Size())

	initScript := `#!/bin/sh
echo "UBUNTU_KERNEL_BOOTED"
/bin/mount -t proc proc /proc 2>/dev/null
/bin/mount -t sysfs sys /sys 2>/dev/null
echo "KERNEL_VERSION: $(cat /proc/version 2>/dev/null | head -1)"
echo "UBUNTU_READY"
/bin/reboot -f
`
	initrdPath := buildBootableInitrd(t, initScript)
	serialLog := filepath.Join(t.TempDir(), "serial-ubuntu.log")

	qemuArgs := []string{
		"-kernel", kernelPath,
		"-initrd", initrdPath,
		"-append", "console=ttyS0",
		"-nographic", "-no-reboot",
		"-m", "256M", "-smp", "1",
		"-serial", "file:" + serialLog,
	}

	t.Logf("Booting Ubuntu kernel from %s...", kernelRef)
	cmd := exec.Command("qemu-system-x86_64", qemuArgs...)
	if err := cmd.Start(); err != nil {
		t.Fatalf("QEMU start: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		t.Fatal("Ubuntu kernel boot timed out after 60s")
	}

	serialStr := string(readFileOrEmpty(serialLog))
	t.Logf("Ubuntu VM serial output:\n%s", serialStr)

	if !strings.Contains(serialStr, "UBUNTU_KERNEL_BOOTED") {
		t.Fatal("Ubuntu kernel did not boot — UBUNTU_KERNEL_BOOTED missing from serial output")
	}

	if !strings.Contains(serialStr, "UBUNTU_READY") {
		t.Fatal("Ubuntu kernel init did not complete — UBUNTU_READY missing")
	}

	if strings.Contains(serialStr, "KERNEL_VERSION:") {
		for _, line := range strings.Split(serialStr, "\n") {
			if strings.Contains(line, "KERNEL_VERSION:") {
				t.Logf("Ubuntu kernel version: %s", strings.TrimSpace(line))
			}
		}
	}

	if strings.Contains(serialStr, "Ubuntu") || strings.Contains(serialStr, "ubuntu") {
		t.Log("Ubuntu kernel identity detected in kernel version string")
	}

	rootfsPath := paths.ImageLayersDir(img.ID)
	t.Logf("Ubuntu rootfs assembled at: %s", rootfsPath)

	t.Logf("SUCCESS: Ubuntu LTS + Ubuntu kernel (%s) boot verified", loaded.KernelRef)

	cleanupImage(t, imgStore, tag, img.ID)
}
