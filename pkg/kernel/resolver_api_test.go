package kernel

import (
	"testing"
)

func TestResolveDebianPackage_Real(t *testing.T) {
	ref, err := ResolveDebianPackage("6.1.0-50-amd64", "amd64")
	if err != nil {
		t.Fatalf("Debian madison API failed (network or API changed): %v", err)
	}
	if ref == "" {
		t.Fatal("expected non-empty resolved reference")
	}
	t.Logf("Deploy: Resolved debian kernel: %s", ref)
}

func TestResolveAlpinePackage_Real(t *testing.T) {
	ref, err := ResolveAlpinePackage("3.21", "lts", "x86_64")
	if err != nil {
		t.Fatalf("Alpine packages API failed (network or API changed): %v", err)
	}
	if ref == "" {
		t.Fatal("expected non-empty resolved reference")
	}
	t.Logf("Alpine: Resolved alpine kernel: %s", ref)
}

func TestResolveArchPackage_Real(t *testing.T) {
	ref, err := ResolveArchPackage("6.9.9")
	if err != nil {
		t.Fatalf("Arch archive API failed (network or API changed): %v", err)
	}
	if ref == "" {
		t.Fatal("expected non-empty resolved reference")
	}
	t.Logf("Arch: Resolved arch kernel: %s", ref)
}

func TestDebianResolver_AutoResolve(t *testing.T) {
	r := NewDebianResolver()
	req := &ResolveRequest{Distro: "debian", Version: "6.1.0-50-amd64", Arch: "amd64"}
	url, format, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("Debian auto-resolution: %v", err)
	}
	if url == "" || format == "" {
		t.Fatalf("expected non-empty url and format, got url=%q format=%q", url, format)
	}
	t.Logf("Debian URL: %s", url)
}

func TestAlpineResolver_AutoResolveReleaseOnly(t *testing.T) {
	r := NewAlpineResolver()
	req := &ResolveRequest{Distro: "alpine", Version: "3.21", Arch: "x86_64"}
	url, format, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("Alpine auto-resolution (release only): %v", err)
	}
	if url == "" || format != "apk" {
		t.Fatalf("expected non-empty url and apk format, got url=%q format=%q", url, format)
	}
	t.Logf("Alpine URL: %s", url)
}

func TestAlpineResolver_AutoResolveReleaseFlavor(t *testing.T) {
	r := NewAlpineResolver()
	req := &ResolveRequest{Distro: "alpine", Version: "3.21:lts", Arch: "x86_64"}
	url, format, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("Alpine auto-resolution (release+flavor): %v", err)
	}
	if url == "" || format != "apk" {
		t.Fatalf("expected non-empty url and apk format, got url=%q format=%q", url, format)
	}
	t.Logf("Alpine URL: %s", url)
}

func TestArchResolver_AutoResolve(t *testing.T) {
	r := NewArchLinuxResolver()
	req := &ResolveRequest{Distro: "archlinux", Version: "6.9.9", Arch: "x86_64"}
	url, format, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("Arch auto-resolution: %v", err)
	}
	if url == "" || format == "" {
		t.Fatalf("expected non-empty url and format, got url=%q format=%q", url, format)
	}
	t.Logf("Arch URL: %s", url)
}

func TestDebianResolver_ExplicitVersion(t *testing.T) {
	r := NewDebianResolver()
	req := &ResolveRequest{Distro: "debian", Version: "6.1.0-50-amd64:6.1.176-1", Arch: "amd64"}
	url, format, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("explicit version should work: %v", err)
	}
	if url == "" || format != "deb" {
		t.Fatalf("expected url and deb format, got url=%q format=%q", url, format)
	}
}
