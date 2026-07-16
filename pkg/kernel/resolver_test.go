package kernel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolverRegistry_KnownDistros(t *testing.T) {
	reg := NewResolverRegistry()

	tests := []struct {
		distro string
		wantOK bool
	}{
		{"debian", true},
		{"alpine", true},
		{"archlinux", true},
		{"oci", true},
		{"http", true},
		{"https", true},
		{"ubuntu", false},
		{"fedora", false},
	}

	for _, tt := range tests {
		req := &ResolveRequest{Distro: tt.distro, Version: "1.0", Arch: "amd64"}
		resolver, _, _, err := reg.Resolve(req)
		if tt.wantOK {
			if err != nil {
				t.Logf("resolver %q returned advice: %v", tt.distro, err)
			}
			if resolver == nil {
				t.Errorf("expected non-nil resolver for %q", tt.distro)
			}
		} else {
			if err == nil {
				t.Errorf("expected error for %q", tt.distro)
			}
		}
	}
}

func TestDebianResolver_RequiresPkgVersion(t *testing.T) {
	r := NewDebianResolver()
	req := &ResolveRequest{Distro: "debian", Version: "6.1.0-25-amd64", Arch: "amd64"}
	_, _, err := r.Resolve(req)
	if err == nil {
		t.Error("expected error for missing package version")
	}
}

func TestDebianResolver_WithPkgVersion(t *testing.T) {
	r := NewDebianResolver()
	req := &ResolveRequest{Distro: "debian", Version: "6.1.0-25-amd64:6.1.106-3", Arch: "amd64"}
	url, format, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if format != "deb" {
		t.Errorf("expected format deb, got %s", format)
	}
	if url == "" {
		t.Error("expected non-empty URL")
	}
}

func TestAlpineResolver_RequiresReleaseAndVersion(t *testing.T) {
	r := NewAlpineResolver()
	req := &ResolveRequest{Distro: "alpine", Version: "3.21", Arch: "x86_64"}
	_, _, err := r.Resolve(req)
	if err == nil {
		t.Error("expected error for missing flavor/version")
	}
}

func TestAlpineResolver_WithFullVersion(t *testing.T) {
	r := NewAlpineResolver()
	req := &ResolveRequest{Distro: "alpine", Version: "3.21:lts:6.6.52-0-lts", Arch: "x86_64"}
	url, format, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if format != "apk" {
		t.Errorf("expected format apk, got %s", format)
	}
	if url == "" {
		t.Error("expected non-empty URL")
	}
}

func TestArchLinuxResolver_RequiresPkgVersion(t *testing.T) {
	r := NewArchLinuxResolver()
	req := &ResolveRequest{Distro: "archlinux", Version: "6.10.10", Arch: "x86_64"}
	_, _, err := r.Resolve(req)
	if err == nil {
		t.Error("expected error for missing package version")
	}
}

func TestArchLinuxResolver_WithPkgVersion(t *testing.T) {
	r := NewArchLinuxResolver()
	req := &ResolveRequest{Distro: "archlinux", Version: "6.10.10:arch1-1", Arch: "x86_64"}
	url, format, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if format != "tar.zst" {
		t.Errorf("expected format tar.zst, got %s", format)
	}
	if url == "" {
		t.Error("expected non-empty URL")
	}
}

func TestHTTPResolver(t *testing.T) {
	r := &HTTPResolver{}

	tests := []struct {
		url    string
		format string
	}{
		{"https://example.com/kernel.tar.gz", "tar.gz"},
		{"https://example.com/kernel.tar.xz", "tar.xz"},
		{"https://example.com/kernel.deb", "deb"},
		{"https://example.com/kernel.apk", "apk"},
	}

	for _, tt := range tests {
		req := &ResolveRequest{Distro: "https", Version: tt.url}
		u, format, err := r.Resolve(req)
		if err != nil {
			t.Errorf("resolve %q: %v", tt.url, err)
			continue
		}
		if u != tt.url {
			t.Errorf("expected url %q, got %q", tt.url, u)
		}
		if format != tt.format {
			t.Errorf("expected format %q, got %q", tt.format, format)
		}
	}
}

func TestHostArchKernel(t *testing.T) {
	arch := hostArchKernel()
	if arch == "" {
		t.Error("expected non-empty arch")
	}
}

func TestFindKernelBinary(t *testing.T) {
	t.Run("BootVmlinuz", func(t *testing.T) {
		dir := t.TempDir()
		bootDir := filepath.Join(dir, "boot")
		os.MkdirAll(bootDir, 0o755)
		os.WriteFile(filepath.Join(bootDir, "vmlinuz-6.1.0"), []byte("kernel"), 0o644)

		path, err := findKernelBinary(dir)
		if err != nil {
			t.Fatalf("findKernelBinary: %v", err)
		}
		if path == "" {
			t.Error("expected non-empty path")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		dir := t.TempDir()
		_, err := findKernelBinary(dir)
		if err == nil {
			t.Error("expected error when no kernel found")
		}
	})
}
