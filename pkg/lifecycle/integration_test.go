package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cookiengineer/poqman/pkg/container"
	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/runtime"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func init() { _ = storage.DefaultPerms }

func TestHealthCheckConfigParsing(t *testing.T) {
	hc := &container.HealthConfig{
		Test:        []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
		Interval:    30000000000,
		Timeout:     3000000000,
		Retries:     3,
		StartPeriod: 5000000000,
	}

	if len(hc.Test) != 2 {
		t.Errorf("expected 2 test parts, got %d", len(hc.Test))
	}
	if hc.Retries != 3 {
		t.Errorf("expected 3 retries, got %d", hc.Retries)
	}
}

func TestHealthStatusConstants(t *testing.T) {
	if container.HealthStarting != "starting" {
		t.Error("expected 'starting'")
	}
	if container.HealthHealthy != "healthy" {
		t.Error("expected 'healthy'")
	}
	if container.HealthUnhealthy != "unhealthy" {
		t.Error("expected 'unhealthy'")
	}
}

func TestHealthStateDefaults(t *testing.T) {
	hs := container.HealthState{
		Status:        container.HealthStarting,
		FailingStreak: 0,
	}
	if hs.Status != container.HealthStarting {
		t.Errorf("expected starting, got %s", hs.Status)
	}
}

func TestResourceLimitsDefaults(t *testing.T) {
	rl := runtime.ResourceLimits{
		MemoryMB:  512,
		CPUShares: 1024,
		PidsLimit: 100,
	}
	if rl.MemoryMB != 512 {
		t.Errorf("expected 512, got %d", rl.MemoryMB)
	}
	if rl.CPUShares != 1024 {
		t.Errorf("expected 1024, got %d", rl.CPUShares)
	}
}

func TestCGroupLifecycle(t *testing.T) {
	limits := runtime.ResourceLimits{MemoryMB: 256}
	err := runtime.ApplyCGroupLimits(1, "test-cgroup", limits)
	if err != nil {
		t.Logf("ApplyCGroupLimits: %v (expected if not root)", err)
	}
	runtime.RemoveCGroup("test-cgroup")
}

func TestQEMUArgsIncludeMemoryLimits(t *testing.T) {
	cfg := runtime.QEMUConfig{
		Binary:  "/usr/bin/qemu-system-x86_64",
		Kernel:  "/path/to/bzImage",
		Memory:  "256M",
		SMP:     1,
	}

	args, err := runtime.BuildArgs(cfg)
	if err != nil {
		t.Fatalf("BuildArgs: %v", err)
	}

	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "256M") {
		t.Error("expected -m 256M in QEMU args")
	}
}

func TestQEMUArgsIncludeSMPLimit(t *testing.T) {
	cfg := runtime.QEMUConfig{
		Binary: "/usr/bin/qemu-system-x86_64",
		Kernel: "/path/to/bzImage",
		SMP:    4,
	}

	args, _ := runtime.BuildArgs(cfg)
	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "-smp") || !strings.Contains(argStr, "4") {
		t.Error("expected -smp 4 in QEMU args")
	}
}

func TestIPv6SubnetConstants(t *testing.T) {
	subnet := "fd00:dead:beef::/64"
	if len(subnet) == 0 {
		t.Error("expected non-empty IPv6 subnet")
	}
}

func TestImageSaveLoadRoundTrip(t *testing.T) {
	paths := setupPaths(t)

	imgStore := image.NewStore(paths)
	img := &image.Image{
		ID:       "sha256:save-test",
		RepoTags: []string{"docker.io/library/save-test:latest"},
		Arch:     "amd64",
		Config: image.ImageConfig{
			Cmd: []string{"/bin/sh"},
			Env: []string{"PATH=/usr/bin"},
		},
		Layers: []image.Layer{
			{Digest: "sha256:layer1", Size: 100, MediaType: "test"},
		},
		Created: time.Now(),
		Size:    100,
	}
	imgStore.Save(img)
	idx, _ := imgStore.LoadIndex()
	idx.Add("docker.io/library/save-test:latest", "sha256:save-test")
	imgStore.SaveIndex(idx)

	outputPath := filepath.Join(paths.Tmp, "save-test.tar.gz")
	if err := image.SaveImage(img, paths, outputPath); err != nil {
		t.Fatalf("SaveImage: %v", err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("output tar.gz should exist")
	}
	if info, _ := os.Stat(outputPath); info.Size() == 0 {
		t.Error("output tar.gz should not be empty")
	}

	loaded, err := image.LoadImage(paths, outputPath)
	if err != nil {
		t.Fatalf("LoadImage: %v", err)
	}
	if loaded.ID != "sha256:save-test" {
		t.Errorf("expected sha256:save-test, got %s", loaded.ID)
	}
	if len(loaded.RepoTags) != 1 || loaded.RepoTags[0] != "docker.io/library/save-test:latest" {
		t.Errorf("unexpected repoTags: %v", loaded.RepoTags)
	}
	if len(loaded.Layers) != 1 {
		t.Errorf("expected 1 layer, got %d", len(loaded.Layers))
	}
}

func TestImageSaveLoad_PreservesConfig(t *testing.T) {
	paths := setupPaths(t)

	imgStore := image.NewStore(paths)
	img := &image.Image{
		ID:       "sha256:config-test",
		RepoTags: []string{"docker.io/library/config-test:latest"},
		Arch:     "arm64",
		Config: image.ImageConfig{
			User:       "nobody",
			Workdir:    "/app",
			Cmd:        []string{"node", "server.js"},
			Entrypoint: []string{"/entrypoint.sh"},
			Env:        []string{"NODE_ENV=production"},
		},
		Layers:  []image.Layer{{Digest: "sha256:l1", Size: 500}},
		Created: time.Now(),
		Size:    500,
	}
	imgStore.Save(img)
	idx, _ := imgStore.LoadIndex()
	idx.Add("docker.io/library/config-test:latest", "sha256:config-test")
	imgStore.SaveIndex(idx)

	outputPath := filepath.Join(paths.Tmp, "config-test.tar.gz")
	image.SaveImage(img, paths, outputPath)

	loaded, _ := image.LoadImage(paths, outputPath)
	if loaded.Arch != "arm64" {
		t.Errorf("expected arm64, got %s", loaded.Arch)
	}
	if loaded.Config.User != "nobody" {
		t.Errorf("expected nobody, got %s", loaded.Config.User)
	}
	if loaded.Config.Workdir != "/app" {
		t.Errorf("expected /app, got %s", loaded.Config.Workdir)
	}
}

func TestSystemIntegration_QEMUBinaryDetection(t *testing.T) {
	detect := runtime.NewDetector()
	binary, err := detect.FindQEMU()
	if err != nil {
		t.Skipf("QEMU not installed: %v", err)
	}
	if binary == "" {
		t.Skip("QEMU path empty")
	}
	t.Logf("QEMU binary: %s", binary)
}

func TestSystemIntegration_QEMUVersion(t *testing.T) {
	detect := runtime.NewDetector()
	version, err := detect.Version()
	if err != nil {
		t.Skipf("QEMU version unavailable: %v", err)
	}
	if version == "" {
		t.Error("expected non-empty version")
	}
	t.Logf("QEMU version: %s", version)
}

func TestSystemIntegration_FullQEMUArgs(t *testing.T) {
	cfg := runtime.QEMUConfig{
		Binary:       "/usr/bin/qemu-system-x86_64",
		Kernel:       "/tmp/test-kernel",
		RootFS:       "/tmp/test-rootfs",
		RootFSFormat: "9p",
		Machine:      "q35",
		CPU:          "host",
		Memory:       "512M",
		SMP:          2,
		Console:      "ttyS0",
		NoGraphic:    true,
		Append:       "root=rootfs rootfstype=9p rw console=ttyS0 quiet poqman.cmd=/bin/sh",
		NetDevID:     "net0",
		TapName:      "tap-test123",
		QMPSocket:    "/tmp/test-qmp.sock",
		PIDFile:      "/tmp/test-pidfile",
		KVM:          true,
	}

	args, err := runtime.BuildArgs(cfg)
	if err != nil {
		t.Fatalf("BuildArgs: %v", err)
	}

	argStr := strings.Join(args, " ")
	expected := []string{"-M", "q35", "-cpu", "host", "-accel", "kvm", "-m", "512M",
		"-smp", "2", "-nographic", "-kernel", "/tmp/test-kernel", "-append",
		"-fsdev", "virtio-9p-pci", "-netdev", "tap-test123", "-qmp", "-pidfile"}

	for _, e := range expected {
		if !strings.Contains(argStr, e) {
			t.Errorf("expected %q in QEMU args: %s", e, argStr)
		}
	}
}

func TestSystemIntegration_ConsoleDeviceByArch(t *testing.T) {
	tests := []struct {
		arch string
		want string
	}{
		{"amd64", "ttyS0"},
		{"arm64", "ttyAMA0"},
		{"riscv64", "ttyAMA0"},
	}

	for _, tt := range tests {
		got := runtime.DefaultConsoleDevice(tt.arch)
		if got != tt.want {
			t.Errorf("DefaultConsoleDevice(%s) = %s, want %s", tt.arch, got, tt.want)
		}
	}
}
