package registry

import (
	"testing"
)

func TestHostPlatform_ReturnsNonEmpty(t *testing.T) {
	p := HostPlatform()
	if p.OS == "" {
		t.Error("expected non-empty OS")
	}
	if p.Architecture == "" {
		t.Error("expected non-empty Architecture")
	}
	if p.OS != "linux" {
		t.Errorf("expected OS linux, got %s", p.OS)
	}
}

func TestParsePlatform_Simple(t *testing.T) {
	p, err := ParsePlatform("linux/amd64")
	if err != nil {
		t.Fatalf("ParsePlatform: %v", err)
	}
	if p.OS != "linux" {
		t.Errorf("expected OS linux, got %s", p.OS)
	}
	if p.Architecture != "amd64" {
		t.Errorf("expected amd64, got %s", p.Architecture)
	}
	if p.Variant != "" {
		t.Errorf("expected empty variant, got %s", p.Variant)
	}
}

func TestParsePlatform_WithVariant(t *testing.T) {
	p, err := ParsePlatform("linux/arm/v7")
	if err != nil {
		t.Fatalf("ParsePlatform: %v", err)
	}
	if p.Architecture != "arm" {
		t.Errorf("expected arm, got %s", p.Architecture)
	}
	if p.Variant != "v7" {
		t.Errorf("expected v7, got %s", p.Variant)
	}
}

func TestParsePlatform_Empty(t *testing.T) {
	p, err := ParsePlatform("")
	if err != nil {
		t.Fatalf("ParsePlatform: %v", err)
	}
	if p.OS != "linux" {
		t.Errorf("expected linux for default, got %s", p.OS)
	}
}

func TestParsePlatform_Invalid(t *testing.T) {
	_, err := ParsePlatform("invalid")
	if err == nil {
		t.Error("expected error for invalid platform string")
	}
}

func TestPlatform_String(t *testing.T) {
	tests := []struct {
		platform Platform
		want     string
	}{
		{Platform{OS: "linux", Architecture: "amd64"}, "linux/amd64"},
		{Platform{OS: "linux", Architecture: "arm", Variant: "v7"}, "linux/arm/v7"},
		{Platform{OS: "linux", Architecture: "arm64"}, "linux/arm64"},
	}

	for _, tt := range tests {
		if got := tt.platform.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestPlatform_Match_Exact(t *testing.T) {
	p1 := Platform{OS: "linux", Architecture: "amd64"}
	p2 := Platform{OS: "linux", Architecture: "amd64"}
	if !p1.Match(p2) {
		t.Error("identical platforms should match")
	}
}

func TestPlatform_Match_DifferentArch(t *testing.T) {
	p1 := Platform{OS: "linux", Architecture: "amd64"}
	p2 := Platform{OS: "linux", Architecture: "arm64"}
	if p1.Match(p2) {
		t.Error("different architectures should not match")
	}
}

func TestPlatform_Match_DifferentOS(t *testing.T) {
	p1 := Platform{OS: "linux", Architecture: "amd64"}
	p2 := Platform{OS: "windows", Architecture: "amd64"}
	if p1.Match(p2) {
		t.Error("different OS should not match")
	}
}

func TestPlatform_Match_Subset(t *testing.T) {
	p1 := Platform{OS: "", Architecture: "amd64"}
	p2 := Platform{OS: "linux", Architecture: "amd64"}
	if !p1.Match(p2) {
		t.Error("empty OS should match any OS")
	}
}

func TestMatchManifest_FindAmd64(t *testing.T) {
	entries := []ManifestListEntry{
		{Digest: "sha256:1", Platform: &Platform{OS: "linux", Architecture: "arm64"}},
		{Digest: "sha256:2", Platform: &Platform{OS: "linux", Architecture: "amd64"}},
		{Digest: "sha256:3", Platform: &Platform{OS: "linux", Architecture: "arm", Variant: "v7"}},
	}

	target := Platform{OS: "linux", Architecture: "amd64"}
	entry, found := MatchManifest(entries, target)
	if !found {
		t.Fatal("expected to find amd64 manifest")
	}
	if entry.Digest != "sha256:2" {
		t.Errorf("expected sha256:2, got %s", entry.Digest)
	}
}

func TestMatchManifest_FallbackVariant(t *testing.T) {
	entries := []ManifestListEntry{
		{Digest: "sha256:1", Platform: &Platform{OS: "linux", Architecture: "arm"}},
	}

	target := Platform{OS: "linux", Architecture: "arm", Variant: "v7"}
	entry, found := MatchManifest(entries, target)
	if !found {
		t.Fatal("expected to find arm manifest via fallback")
	}
	if entry.Digest != "sha256:1" {
		t.Errorf("expected sha256:1, got %s", entry.Digest)
	}
}

func TestMatchManifest_NotFound(t *testing.T) {
	entries := []ManifestListEntry{
		{Digest: "sha256:1", Platform: &Platform{OS: "linux", Architecture: "amd64"}},
	}

	target := Platform{OS: "windows", Architecture: "amd64"}
	_, found := MatchManifest(entries, target)
	if found {
		t.Error("should not find windows manifest in linux-only list")
	}
}

func TestMatchManifest_NoPlatformEntry(t *testing.T) {
	entries := []ManifestListEntry{
		{Digest: "sha256:1", Platform: nil},
	}

	target := Platform{OS: "linux", Architecture: "amd64"}
	_, found := MatchManifest(entries, target)
	if found {
		t.Error("should not match entry with no platform")
	}
}
