package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePaths_DefaultBase(t *testing.T) {
	paths, err := ResolvePaths()
	if err != nil {
		t.Fatalf("ResolvePaths: %v", err)
	}
	if paths.Base == "" {
		t.Error("expected non-empty base path")
	}
	if paths.Images == "" || paths.Kernels == "" || paths.Containers == "" {
		t.Error("expected all sub-paths to be set")
	}
}

func TestResolvePaths_XDGDataHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	paths, err := ResolvePaths()
	if err != nil {
		t.Fatalf("ResolvePaths: %v", err)
	}
	expected := filepath.Join(tmp, AppName)
	if paths.Base != expected {
		t.Errorf("expected base %s, got %s", expected, paths.Base)
	}
}

func TestPaths_EnsureAll(t *testing.T) {
	tmp := t.TempDir()
	paths := &Paths{
		Base:       tmp,
		Images:     filepath.Join(tmp, "images"),
		Kernels:    filepath.Join(tmp, "kernels"),
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
		Tmp:        filepath.Join(tmp, "tmp"),
	}

	if err := paths.EnsureAll(); err != nil {
		t.Fatalf("EnsureAll: %v", err)
	}

	for _, dir := range []string{paths.Images, paths.Kernels, paths.Containers, paths.Networks, paths.Tmp} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("expected directory %s to exist", dir)
		}
	}
}

func TestPaths_ImagePaths(t *testing.T) {
	paths := &Paths{
		Base:       "/tmp/poqman-test",
		Images:     "/tmp/poqman-test/images",
		Kernels:    "/tmp/poqman-test/kernels",
		Containers: "/tmp/poqman-test/containers",
		Networks:   "/tmp/poqman-test/networks",
		Tmp:        "/tmp/poqman-test/tmp",
	}

	if p := paths.ImageIndexPath(); p != "/tmp/poqman-test/images/index.json" {
		t.Errorf("ImageIndexPath: %s", p)
	}
	if p := paths.ImagePath("sha256:abc"); p != "/tmp/poqman-test/images/sha256:abc" {
		t.Errorf("ImagePath: %s", p)
	}
	if p := paths.ImageManifestPath("sha256:abc"); p != "/tmp/poqman-test/images/sha256:abc/manifest.json" {
		t.Errorf("ImageManifestPath: %s", p)
	}
	if p := paths.ImageConfigPath("sha256:abc"); p != "/tmp/poqman-test/images/sha256:abc/config.json" {
		t.Errorf("ImageConfigPath: %s", p)
	}
	if p := paths.ImageLayersDir("sha256:abc"); p != "/tmp/poqman-test/images/sha256:abc/layers" {
		t.Errorf("ImageLayersDir: %s", p)
	}
	if p := paths.ImageLayerPath("sha256:abc", "sha256:layer1"); p != "/tmp/poqman-test/images/sha256:abc/layers/sha256:layer1" {
		t.Errorf("ImageLayerPath: %s", p)
	}
	if p := paths.ImageKernelPath("sha256:abc"); p != "/tmp/poqman-test/images/sha256:abc/kernel/bzImage" {
		t.Errorf("ImageKernelPath: %s", p)
	}
}

func TestPaths_ContainerPaths(t *testing.T) {
	paths := &Paths{
		Base:       "/tmp/poqman-test",
		Images:     "/tmp/poqman-test/images",
		Kernels:    "/tmp/poqman-test/kernels",
		Containers: "/tmp/poqman-test/containers",
		Networks:   "/tmp/poqman-test/networks",
		Tmp:        "/tmp/poqman-test/tmp",
	}

	if p := paths.ContainerPath("abc123"); p != "/tmp/poqman-test/containers/abc123" {
		t.Errorf("ContainerPath: %s", p)
	}
	if p := paths.ContainerConfigPath("abc123"); p != "/tmp/poqman-test/containers/abc123/config.json" {
		t.Errorf("ContainerConfigPath: %s", p)
	}
	if p := paths.ContainerStatePath("abc123"); p != "/tmp/poqman-test/containers/abc123/state.json" {
		t.Errorf("ContainerStatePath: %s", p)
	}
	if p := paths.ContainerRootfsPath("abc123"); p != "/tmp/poqman-test/containers/abc123/rootfs" {
		t.Errorf("ContainerRootfsPath: %s", p)
	}
	if p := paths.ContainerConsoleLogPath("abc123"); p != "/tmp/poqman-test/containers/abc123/console.log" {
		t.Errorf("ContainerConsoleLogPath: %s", p)
	}
	if p := paths.ContainerPIDFilePath("abc123"); p != "/tmp/poqman-test/containers/abc123/pidfile" {
		t.Errorf("ContainerPIDFilePath: %s", p)
	}
	if p := paths.ContainerQMPSocketPath("abc123"); p != "/tmp/poqman-test/containers/abc123/qmp.sock" {
		t.Errorf("ContainerQMPSocketPath: %s", p)
	}
	if p := paths.ContainerAgentSocketPath("abc123"); p != "/tmp/poqman-test/containers/abc123/agent.sock" {
		t.Errorf("ContainerAgentSocketPath: %s", p)
	}
}

func TestPaths_KernelPaths(t *testing.T) {
	paths := &Paths{
		Base:       "/tmp/poqman-test",
		Images:     "/tmp/poqman-test/images",
		Kernels:    "/tmp/poqman-test/kernels",
		Containers: "/tmp/poqman-test/containers",
		Networks:   "/tmp/poqman-test/networks",
		Tmp:        "/tmp/poqman-test/tmp",
	}

	if p := paths.KernelIndexPath(); p != "/tmp/poqman-test/kernels/index.json" {
		t.Errorf("KernelIndexPath: %s", p)
	}
	if p := paths.KernelPath("sha256:kernel1"); p != "/tmp/poqman-test/kernels/sha256:kernel1" {
		t.Errorf("KernelPath: %s", p)
	}
	if p := paths.KernelImagePath("sha256:kernel1"); p != "/tmp/poqman-test/kernels/sha256:kernel1/bzImage" {
		t.Errorf("KernelImagePath: %s", p)
	}
	if p := paths.KernelConfigPath("sha256:kernel1"); p != "/tmp/poqman-test/kernels/sha256:kernel1/config.json" {
		t.Errorf("KernelConfigPath: %s", p)
	}
}

func TestPaths_NetworkPath(t *testing.T) {
	paths := &Paths{
		Base:       "/tmp/poqman-test",
		Images:     "/tmp/poqman-test/images",
		Kernels:    "/tmp/poqman-test/kernels",
		Containers: "/tmp/poqman-test/containers",
		Networks:   "/tmp/poqman-test/networks",
		Tmp:        "/tmp/poqman-test/tmp",
	}

	if p := paths.NetworkStatePath(); p != "/tmp/poqman-test/networks/state.json" {
		t.Errorf("NetworkStatePath: %s", p)
	}
}
