package runtime

import (
	"testing"
)

func TestDetector_GetArchSpec(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		goarch string
		want   string
	}{
		{"amd64", "qemu-system-x86_64"},
		{"arm64", "qemu-system-aarch64"},
		{"arm", "qemu-system-arm"},
		{"riscv64", "qemu-system-riscv64"},
		{"ppc64le", "qemu-system-ppc64"},
	}

	for _, tt := range tests {
		arch, err := d.GetArchSpec(tt.goarch)
		if err != nil {
			t.Errorf("GetArchSpec(%q): %v", tt.goarch, err)
			continue
		}
		if arch.QEMUBinary != tt.want {
			t.Errorf("GetArchSpec(%q).QEMUBinary = %q, want %q", tt.goarch, arch.QEMUBinary, tt.want)
		}
	}
}

func TestDetector_GetArchSpec_Empty(t *testing.T) {
	d := NewDetector()
	arch, err := d.GetArchSpec("")
	if err != nil {
		t.Fatalf("GetArchSpec(empty): %v", err)
	}
	if arch.GoArch != d.hostArch {
		t.Errorf("expected host arch %s, got %s", d.hostArch, arch.GoArch)
	}
}

func TestDetector_GetArchSpec_Unknown(t *testing.T) {
	d := NewDetector()
	_, err := d.GetArchSpec("unknown-arch")
	if err == nil {
		t.Error("expected error for unknown arch")
	}
}

func TestDetector_HostArchSpec(t *testing.T) {
	d := NewDetector()
	arch, err := d.HostArchSpec()
	if err != nil {
		t.Fatalf("HostArchSpec: %v", err)
	}
	if arch.GoArch == "" {
		t.Error("expected non-empty GoArch")
	}
}

func TestDetector_BinaryForArch(t *testing.T) {
	d := NewDetector()

	_, err := d.BinaryForArch("unknown-arch")
	if err == nil {
		t.Error("expected error for unknown arch")
	}
}

func TestDefaultConsoleDevice(t *testing.T) {
	tests := []struct {
		arch string
		want string
	}{
		{"amd64", "ttyS0"},
		{"arm64", "ttyAMA0"},
		{"arm", "ttyAMA0"},
		{"riscv64", "ttyAMA0"},
	}

	for _, tt := range tests {
		if got := DefaultConsoleDevice(tt.arch); got != tt.want {
			t.Errorf("DefaultConsoleDevice(%q) = %q, want %q", tt.arch, got, tt.want)
		}
	}
}
