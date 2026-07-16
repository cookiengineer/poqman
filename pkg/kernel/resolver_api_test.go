package kernel

import (
	"testing"
)

func TestResolveDebianPackage_Real(t *testing.T) {
	ref, err := ResolveDebianPackage("6.1.0-25-amd64", "amd64")
	if err != nil {
		t.Skipf("Debian madison API unavailable (skipping integration test): %v", err)
	}
	if ref == "" {
		t.Error("expected non-empty resolved reference")
	}
	t.Logf("Resolved: %s", ref)
}

func TestResolveAlpinePackage_Real(t *testing.T) {
	ref, err := ResolveAlpinePackage("3.21", "lts", "x86_64")
	if err != nil {
		t.Skipf("Alpine packages API unavailable (skipping integration test): %v", err)
	}
	if ref == "" {
		t.Error("expected non-empty resolved reference")
	}
	t.Logf("Resolved: %s", ref)
}

func TestResolveArchPackage_Real(t *testing.T) {
	ref, err := ResolveArchPackage("6.10.10")
	if err != nil {
		t.Skipf("Arch archive API unavailable (skipping integration test): %v", err)
	}
	if ref == "" {
		t.Error("expected non-empty resolved reference")
	}
	t.Logf("Resolved: %s", ref)
}

func TestDebianResolver_AutoResolve(t *testing.T) {
	r := NewDebianResolver()
	req := &ResolveRequest{Distro: "debian", Version: "6.1.0-25-amd64", Arch: "amd64"}
	url, format, err := r.Resolve(req)
	if err != nil {
		t.Skipf("Debian auto-resolution skipped (API call failed): %v", err)
	}
	if url == "" || format == "" {
		t.Errorf("expected non-empty url and format, got url=%q format=%q", url, format)
	}
	t.Logf("Debian URL: %s", url)
}

func TestAlpineResolver_AutoResolveReleaseOnly(t *testing.T) {
	r := NewAlpineResolver()
	req := &ResolveRequest{Distro: "alpine", Version: "3.21", Arch: "x86_64"}
	url, format, err := r.Resolve(req)
	if err != nil {
		t.Skipf("Alpine auto-resolution skipped (API call failed): %v", err)
	}
	if url == "" || format != "apk" {
		t.Errorf("expected non-empty url and apk format, got url=%q format=%q", url, format)
	}
	t.Logf("Alpine URL: %s", url)
}

func TestAlpineResolver_AutoResolveReleaseFlavor(t *testing.T) {
	r := NewAlpineResolver()
	req := &ResolveRequest{Distro: "alpine", Version: "3.21:lts", Arch: "x86_64"}
	url, format, err := r.Resolve(req)
	if err != nil {
		t.Skipf("Alpine auto-resolution skipped: %v", err)
	}
	if url == "" || format != "apk" {
		t.Errorf("expected non-empty url and apk format, got url=%q format=%q", url, format)
	}
	t.Logf("Alpine URL: %s", url)
}

func TestArchResolver_AutoResolve(t *testing.T) {
	r := NewArchLinuxResolver()
	req := &ResolveRequest{Distro: "archlinux", Version: "6.10.10", Arch: "x86_64"}
	url, format, err := r.Resolve(req)
	if err != nil {
		t.Skipf("Arch auto-resolution skipped (API call failed): %v", err)
	}
	if url == "" || format == "" {
		t.Errorf("expected non-empty url and format, got url=%q format=%q", url, format)
	}
	t.Logf("Arch URL: %s", url)
}

func TestDebianResolver_ExplicitVersion(t *testing.T) {
	r := NewDebianResolver()
	req := &ResolveRequest{Distro: "debian", Version: "6.1.0-25-amd64:6.1.106-3", Arch: "amd64"}
	url, format, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("explicit version should work: %v", err)
	}
	if url == "" || format != "deb" {
		t.Errorf("expected url and deb format, got url=%q format=%q", url, format)
	}
}
