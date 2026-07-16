package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAssembleRootfs(t *testing.T) {
	tmp := t.TempDir()
	paths := &Paths{
		Base:       tmp,
		Images:     filepath.Join(tmp, "images"),
		Kernels:    filepath.Join(tmp, "kernels"),
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
		Tmp:        filepath.Join(tmp, "tmp"),
	}
	paths.EnsureAll()

	layerDir := filepath.Join(paths.ImageLayersDir("img1"), "sha256:layer1")
	os.MkdirAll(filepath.Join(layerDir, "bin"), 0o755)
	os.MkdirAll(filepath.Join(layerDir, "etc"), 0o755)
	os.WriteFile(filepath.Join(layerDir, "bin", "sh"), []byte("binary"), 0o755)
	os.WriteFile(filepath.Join(layerDir, "etc", "config"), []byte("config"), 0o644)

	layer2Dir := filepath.Join(paths.ImageLayersDir("img1"), "sha256:layer2")
	os.MkdirAll(filepath.Join(layer2Dir, "usr", "bin"), 0o755)
	os.MkdirAll(filepath.Join(layer2Dir, "etc"), 0o755)
	os.WriteFile(filepath.Join(layer2Dir, "usr", "bin", "app"), []byte("app binary"), 0o755)
	os.WriteFile(filepath.Join(layer2Dir, "etc", "config"), []byte("overwritten config"), 0o644)

	rootfsPath, err := AssembleRootfs("img1", "container1", paths)
	if err != nil {
		t.Fatalf("AssembleRootfs: %v", err)
	}

	if _, err := os.Stat(rootfsPath); os.IsNotExist(err) {
		t.Fatal("rootfs directory does not exist")
	}

	shContent, err := os.ReadFile(filepath.Join(rootfsPath, "bin", "sh"))
	if err != nil {
		t.Errorf("read bin/sh: %v", err)
	}
	if string(shContent) != "binary" {
		t.Errorf("expected 'binary', got %q", string(shContent))
	}

	configContent, err := os.ReadFile(filepath.Join(rootfsPath, "etc", "config"))
	if err != nil {
		t.Errorf("read etc/config: %v", err)
	}
	if string(configContent) != "overwritten config" {
		t.Errorf("expected 'overwritten config', got %q", string(configContent))
	}

	appContent, err := os.ReadFile(filepath.Join(rootfsPath, "usr", "bin", "app"))
	if err != nil {
		t.Errorf("read usr/bin/app: %v", err)
	}
	if string(appContent) != "app binary" {
		t.Errorf("expected 'app binary', got %q", string(appContent))
	}
}

func TestAssembleRootfs_EmptyLayers(t *testing.T) {
	tmp := t.TempDir()
	paths := &Paths{
		Base:       tmp,
		Images:     filepath.Join(tmp, "images"),
		Kernels:    filepath.Join(tmp, "kernels"),
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
		Tmp:        filepath.Join(tmp, "tmp"),
	}
	paths.EnsureAll()

	rootfsPath, err := AssembleRootfs("nonexistent", "container1", paths)
	if err != nil {
		t.Fatalf("AssembleRootfs empty: %v", err)
	}
	if _, err := os.Stat(rootfsPath); os.IsNotExist(err) {
		t.Fatal("rootfs should exist even if empty")
	}
}

func TestInjectInit(t *testing.T) {
	tmp := t.TempDir()
	rootfs := filepath.Join(tmp, "rootfs")
	os.MkdirAll(rootfs, 0o755)

	initBinary := []byte("fake-init-binary")
	if err := InjectInit(rootfs, initBinary); err != nil {
		t.Fatalf("InjectInit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rootfs, "sbin", "init"))
	if err != nil {
		t.Fatalf("read sbin/init: %v", err)
	}
	if string(data) != "fake-init-binary" {
		t.Errorf("expected 'fake-init-binary', got %q", string(data))
	}
}

func TestCopyKernel(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	dstDir := filepath.Join(tmp, "dst")
	os.MkdirAll(srcDir, 0o755)
	srcPath := filepath.Join(srcDir, "vmlinuz-6.1.0")
	os.WriteFile(srcPath, []byte("kernel-content"), 0o644)

	if err := CopyKernel(srcPath, dstDir); err != nil {
		t.Fatalf("CopyKernel: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dstDir, "bzImage"))
	if err != nil {
		t.Fatalf("read bzImage: %v", err)
	}
	if string(data) != "kernel-content" {
		t.Errorf("expected 'kernel-content', got %q", string(data))
	}
}
