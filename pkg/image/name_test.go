package image

import (
	"testing"
)

func TestParseImageRef_Simple(t *testing.T) {
	ref, err := ParseImageRef("alpine")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Registry != "docker.io" {
		t.Errorf("expected registry docker.io, got %s", ref.Registry)
	}
	if ref.Repository != "library/alpine" {
		t.Errorf("expected repository library/alpine, got %s", ref.Repository)
	}
	if ref.Tag != "latest" {
		t.Errorf("expected tag latest, got %s", ref.Tag)
	}
	if ref.Digest != "" {
		t.Errorf("expected empty digest, got %s", ref.Digest)
	}
}

func TestParseImageRef_WithTag(t *testing.T) {
	ref, err := ParseImageRef("nginx:1.25")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Registry != "docker.io" {
		t.Errorf("expected registry docker.io, got %s", ref.Registry)
	}
	if ref.Repository != "library/nginx" {
		t.Errorf("expected repository library/nginx, got %s", ref.Repository)
	}
	if ref.Tag != "1.25" {
		t.Errorf("expected tag 1.25, got %s", ref.Tag)
	}
}

func TestParseImageRef_WithDigest(t *testing.T) {
	ref, err := ParseImageRef("alpine@sha256:abc123def456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Digest != "sha256:abc123def456" {
		t.Errorf("expected digest sha256:abc123def456, got %s", ref.Digest)
	}
	if ref.Tag != "latest" {
		t.Errorf("expected tag latest, got %s", ref.Tag)
	}
}

func TestParseImageRef_CustomRegistry(t *testing.T) {
	ref, err := ParseImageRef("myregistry.io/team/service:v2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Registry != "myregistry.io" {
		t.Errorf("expected registry myregistry.io, got %s", ref.Registry)
	}
	if ref.Repository != "team/service" {
		t.Errorf("expected repository team/service, got %s", ref.Repository)
	}
	if ref.Tag != "v2" {
		t.Errorf("expected tag v2, got %s", ref.Tag)
	}
}

func TestParseImageRef_LocalhostRegistry(t *testing.T) {
	ref, err := ParseImageRef("localhost:5000/myrepo:dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Registry != "localhost:5000" {
		t.Errorf("expected registry localhost:5000, got %s", ref.Registry)
	}
	if ref.Repository != "myrepo" {
		t.Errorf("expected repository myrepo, got %s", ref.Repository)
	}
	if ref.Tag != "dev" {
		t.Errorf("expected tag dev, got %s", ref.Tag)
	}
}

func TestParseImageRef_EmptyString(t *testing.T) {
	_, err := ParseImageRef("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestImageRef_FullName(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"alpine", "docker.io/library/alpine:latest"},
		{"nginx:1.25", "docker.io/library/nginx:1.25"},
		{"myreg.io/team/app:v1", "myreg.io/team/app:v1"},
		{"alpine@sha256:abc", "docker.io/library/alpine@sha256:abc"},
	}
	for _, tt := range tests {
		ref, err := ParseImageRef(tt.raw)
		if err != nil {
			t.Errorf("parse %q: %v", tt.raw, err)
			continue
		}
		if got := ref.FullName(); got != tt.want {
			t.Errorf("FullName(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestImageRef_String(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"alpine", "library/alpine"},
		{"nginx:1.25", "library/nginx:1.25"},
		{"myreg.io/team/app:v1", "myreg.io/team/app:v1"},
	}
	for _, tt := range tests {
		ref, err := ParseImageRef(tt.raw)
		if err != nil {
			t.Errorf("parse %q: %v", tt.raw, err)
			continue
		}
		if got := ref.String(); got != tt.want {
			t.Errorf("String(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestImageIndex_AddAndLookup(t *testing.T) {
	idx := NewImageIndex()
	idx.Add("docker.io/library/alpine:latest", "sha256:abc")
	id, ok := idx.Lookup("docker.io/library/alpine:latest")
	if !ok {
		t.Fatal("expected to find image")
	}
	if id != "sha256:abc" {
		t.Errorf("expected sha256:abc, got %s", id)
	}
}

func TestImageIndex_Remove(t *testing.T) {
	idx := NewImageIndex()
	idx.Add("docker.io/library/alpine:latest", "sha256:abc")
	idx.Remove("docker.io/library/alpine:latest")
	_, ok := idx.Lookup("docker.io/library/alpine:latest")
	if ok {
		t.Error("expected image to be removed")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID([]byte("test-data-1"))
	id2 := GenerateID([]byte("test-data-2"))
	id3 := GenerateID([]byte("test-data-1"))

	if id1 == id2 {
		t.Error("different inputs should produce different IDs")
	}
	if id1 != id3 {
		t.Error("same inputs should produce same IDs")
	}
	if len(id1) < 20 {
		t.Errorf("ID too short: %s", id1)
	}
}
