package runtime

import (
	"strings"
	"testing"

	"github.com/cookiengineer/poqman/pkg/container"
)

func TestBuildArgs_Basic(t *testing.T) {
	cfg := QEMUConfig{
		Binary:     "/usr/bin/qemu-system-x86_64",
		Kernel:     "/path/to/bzImage",
		Machine:    "q35",
		CPU:        "host",
		Memory:     "512M",
		SMP:        2,
		NoGraphic:  true,
		RootFS:     "/path/to/rootfs",
		Console:    "ttyS0",
		Append:     "root=rootfs rootfstype=9p rw console=ttyS0",
		QMPSocket:  "/tmp/qmp.sock",
		PIDFile:    "/tmp/pidfile",
	}

	args, err := BuildArgs(cfg)
	if err != nil {
		t.Fatalf("BuildArgs: %v", err)
	}

	assertContains(t, args, "-M", "q35")
	assertContains(t, args, "-cpu", "host")
	assertContains(t, args, "-m", "512M")
	assertContains(t, args, "-smp", "2")
	assertContains(t, args, "-nographic")
	assertContains(t, args, "-kernel", "/path/to/bzImage")
	assertContains(t, args, "-append")
}

func TestBuildArgs_Defaults(t *testing.T) {
	cfg := QEMUConfig{
		Binary: "/usr/bin/qemu-system-x86_64",
		Kernel: "/path/to/bzImage",
	}

	args, err := BuildArgs(cfg)
	if err != nil {
		t.Fatalf("BuildArgs: %v", err)
	}

	assertContains(t, args, "-m", "512M")
	assertContains(t, args, "-smp", "2")
	assertContains(t, args, "-display", "none")
}

func TestBuildArgs_WithNetworking(t *testing.T) {
	cfg := QEMUConfig{
		Binary:    "/usr/bin/qemu-system-x86_64",
		Kernel:    "/path/to/bzImage",
		NetDevID:  "net0",
		TapName:   "tap-abc123",
		NetMAC:    "52:54:00:aa:bb:cc",
	}

	args, err := BuildArgs(cfg)
	if err != nil {
		t.Fatalf("BuildArgs: %v", err)
	}

	netdevFound := false
	deviceFound := false
	for _, arg := range args {
		if strings.Contains(arg, "tap,id=net0") {
			netdevFound = true
		}
		if strings.Contains(arg, "virtio-net-pci") && strings.Contains(arg, "netdev=net0") {
			deviceFound = true
		}
	}

	if !netdevFound {
		t.Error("expected netdev tap argument")
	}
	if !deviceFound {
		t.Error("expected virtio-net device argument")
	}
}

func TestBuildArgs_WithVolumes(t *testing.T) {
	cfg := QEMUConfig{
		Binary: "/usr/bin/qemu-system-x86_64",
		Kernel: "/path/to/bzImage",
		VolumeMounts: []container.VolumeMount{
			{Source: "/host/data", Target: "/data"},
			{Source: "/host/config", Target: "/config", ReadOnly: true},
		},
	}

	args, err := BuildArgs(cfg)
	if err != nil {
		t.Fatalf("BuildArgs: %v", err)
	}

	volume0Found := false
	volume1ROFound := false
	for _, arg := range args {
		if strings.Contains(arg, "volume0") && strings.Contains(arg, "/host/data") {
			volume0Found = true
		}
		if strings.Contains(arg, "volume1") && strings.Contains(arg, "readonly=on") {
			volume1ROFound = true
		}
	}

	if !volume0Found {
		t.Error("expected volume0 fsdev entry")
	}
	if !volume1ROFound {
		t.Error("expected volume1 readonly fsdev entry")
	}
}

func TestBuildArgs_MissingBinary(t *testing.T) {
	cfg := QEMUConfig{Kernel: "/path/to/bzImage"}
	_, err := BuildArgs(cfg)
	if err == nil {
		t.Error("expected error for missing binary")
	}
}

func TestBuildArgs_MissingKernel(t *testing.T) {
	cfg := QEMUConfig{Binary: "/usr/bin/qemu-system-x86_64"}
	_, err := BuildArgs(cfg)
	if err == nil {
		t.Error("expected error for missing kernel")
	}
}

func TestBuildArgs_WithKVM(t *testing.T) {
	cfg := QEMUConfig{
		Binary: "/usr/bin/qemu-system-x86_64",
		Kernel: "/path/to/bzImage",
		KVM:    true,
	}

	args, err := BuildArgs(cfg)
	if err != nil {
		t.Fatalf("BuildArgs: %v", err)
	}

	assertContains(t, args, "-accel", "kvm")
}

func TestBuildKernelAppend_Basic(t *testing.T) {
	cfg := QEMUConfig{
		Console:      "ttyS0",
		RootFSFormat: "9p",
	}

	appendLine := BuildKernelAppend(cfg)

	assertSubstring(t, appendLine, "root=rootfs")
	assertSubstring(t, appendLine, "rootfstype=9p")
	assertSubstring(t, appendLine, "trans=virtio")
	assertSubstring(t, appendLine, "console=ttyS0")
}

func TestBuildInitCmdline_Basic(t *testing.T) {
	cmdline := BuildInitCmdline(
		[]string{"/bin/sh", "-c", "echo hello"},
		"test-host",
		"10.88.0.5/16",
		"10.88.0.1",
		"1.1.1.1",
		nil,
	)

	assertSubstring(t, cmdline, "poqman.hostname=")
	assertSubstring(t, cmdline, "poqman.ip=10.88.0.5/16")
	assertSubstring(t, cmdline, "poqman.gateway=10.88.0.1")
	assertSubstring(t, cmdline, "poqman.dns=1.1.1.1")
	assertSubstring(t, cmdline, "poqman.cmd=")
	assertSubstring(t, cmdline, "echo hello")
}

func TestBuildInitCmdline_EmptyCmd(t *testing.T) {
	cmdline := BuildInitCmdline(nil, "", "", "", "", nil)
	assertSubstring(t, cmdline, "poqman.cmd=/sbin/init")
}

func TestBuildInitCmdline_WithVolumes(t *testing.T) {
	volumes := []container.VolumeMount{
		{Source: "/host/data", Target: "/data"},
		{Source: "/host/config", Target: "/config", ReadOnly: true},
	}

	cmdline := BuildInitCmdline(nil, "test", "10.88.0.5", "10.88.0.1", "", volumes)

	assertSubstring(t, cmdline, "poqman.volume.0.source=/host/data")
	assertSubstring(t, cmdline, "poqman.volume.0.target=/data")
	assertSubstring(t, cmdline, "poqman.volume.0.readonly=false")
	assertSubstring(t, cmdline, "poqman.volume.1.source=/host/config")
	assertSubstring(t, cmdline, "poqman.volume.1.target=/config")
	assertSubstring(t, cmdline, "poqman.volume.1.readonly=true")
}

func TestGenerateMAC(t *testing.T) {
	mac := GenerateMAC("abcdef123456")
	if len(mac) != 17 {
		t.Errorf("expected MAC length 17, got %d", len(mac))
	}
	if !strings.HasPrefix(mac, "52:54:00:") {
		t.Errorf("expected MAC prefix 52:54:00:, got %s", mac)
	}

	mac2 := GenerateMAC("fedcba654321")
	if mac == mac2 {
		t.Error("different IDs should produce different MACs")
	}
}

func TestEscapeKernelArg(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"hello world", "\"hello world\""},
		{"with\ttab", "\"with\ttab\""},
		{"with\"quote", "with\\\"quote"},
	}

	for _, tt := range tests {
		got := escapeKernelArg(tt.input)
		if got != tt.want {
			t.Errorf("escapeKernelArg(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func assertContains(t *testing.T, args []string, expected ...string) {
	t.Helper()
	argStr := strings.Join(args, " ")
	for _, exp := range expected {
		if !strings.Contains(argStr, exp) {
			t.Errorf("expected args to contain %q, got: %s", exp, argStr)
		}
	}
}

func assertSubstring(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("expected %q to contain %q", s, sub)
	}
}
