package kernel

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cookiengineer/poqman/pkg/storage"
)

func TestParseKernelRef_Simple(t *testing.T) {
	req, err := ParseKernelRef("debian:6.1.0-25-amd64")
	if err != nil {
		t.Fatalf("ParseKernelRef: %v", err)
	}
	if req.Distro != "debian" {
		t.Errorf("expected distro debian, got %s", req.Distro)
	}
	if req.Version != "6.1.0-25-amd64" {
		t.Errorf("expected version 6.1.0-25-amd64, got %s", req.Version)
	}
	if req.Arch != "" {
		t.Errorf("expected empty arch, got %s", req.Arch)
	}
}

func TestParseKernelRef_WithArch(t *testing.T) {
	req, err := ParseKernelRef("debian:6.1.0-25:amd64")
	if err != nil {
		t.Fatalf("ParseKernelRef: %v", err)
	}
	if req.Distro != "debian" {
		t.Errorf("expected distro debian, got %s", req.Distro)
	}
	if req.Version != "6.1.0-25" {
		t.Errorf("expected version 6.1.0-25, got %s", req.Version)
	}
	if req.Arch != "amd64" {
		t.Errorf("expected arch amd64, got %s", req.Arch)
	}
}

func TestParseKernelRef_Alpine(t *testing.T) {
	req, err := ParseKernelRef("alpine:3.21:lts:6.6.52-0-lts")
	if err != nil {
		t.Fatalf("ParseKernelRef: %v", err)
	}
	if req.Distro != "alpine" {
		t.Errorf("expected distro alpine, got %s", req.Distro)
	}
	if req.Version != "3.21:lts:6.6.52-0-lts" {
		t.Errorf("expected full version, got %s", req.Version)
	}
	if req.Arch != "" {
		t.Errorf("expected empty arch (auto-detect), got %s", req.Arch)
	}
}

func TestParseKernelRef_Empty(t *testing.T) {
	_, err := ParseKernelRef("")
	if err == nil {
		t.Error("expected error for empty ref")
	}
}

func TestParseKernelRef_Invalid(t *testing.T) {
	_, err := ParseKernelRef("justastring")
	if err == nil {
		t.Error("expected error for invalid ref")
	}
}

func TestResolveRequest_String(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"debian:6.1.0-25-amd64", "debian:6.1.0-25-amd64"},
		{"debian:6.1.0-25:amd64", "debian:6.1.0-25:amd64"},
	}

	for _, tt := range tests {
		req, _ := ParseKernelRef(tt.raw)
		if got := req.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestGenerateKernelID(t *testing.T) {
	id1 := GenerateKernelID([]byte("test"))
	id2 := GenerateKernelID([]byte("test"))
	id3 := GenerateKernelID([]byte("different"))

	if id1 != id2 {
		t.Error("same input should produce same ID")
	}
	if id1 == id3 {
		t.Error("different inputs should produce different IDs")
	}
}

func TestKernelIndex_AddLookup(t *testing.T) {
	idx := NewKernelIndex()
	idx.Add("debian:6.1.0-25-amd64", "sha256:abc")
	id, ok := idx.Lookup("debian:6.1.0-25-amd64")
	if !ok {
		t.Fatal("expected to find kernel")
	}
	if id != "sha256:abc" {
		t.Errorf("expected sha256:abc, got %s", id)
	}
}

func TestKernelIndex_Remove(t *testing.T) {
	idx := NewKernelIndex()
	idx.Add("debian:6.1.0-25", "sha256:abc")
	idx.Remove("debian:6.1.0-25")
	_, ok := idx.Lookup("debian:6.1.0-25")
	if ok {
		t.Error("expected kernel to be removed")
	}
}

func TestStore_SaveAndGet(t *testing.T) {
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Images:     filepath.Join(tmp, "images"),
		Kernels:    filepath.Join(tmp, "kernels"),
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
		Tmp:        filepath.Join(tmp, "tmp"),
	}
	paths.EnsureAll()

	store := NewStore(paths)

	k := &Kernel{
		ID:         "sha256:kernel1",
		Distro:     "debian",
		Version:    "6.1.0-25-amd64",
		Arch:       "amd64",
		PackageURL: "http://example.com/kernel.deb",
		Created:    time.Now(),
	}

	if err := store.Save(k); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Get("sha256:kernel1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded.Distro != "debian" {
		t.Errorf("expected distro debian, got %s", loaded.Distro)
	}
	if loaded.Version != "6.1.0-25-amd64" {
		t.Errorf("expected version 6.1.0-25-amd64, got %s", loaded.Version)
	}
}

func TestStore_Resolve(t *testing.T) {
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Images:     filepath.Join(tmp, "images"),
		Kernels:    filepath.Join(tmp, "kernels"),
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
		Tmp:        filepath.Join(tmp, "tmp"),
	}
	paths.EnsureAll()

	store := NewStore(paths)

	k := &Kernel{
		ID:      "sha256:deb-kernel",
		Distro:  "debian",
		Version: "6.1.0-25",
		Arch:    "amd64",
		Created: time.Now(),
	}
	store.Save(k)

	idx, _ := store.LoadIndex()
	idx.Add("debian:6.1.0-25:amd64", "sha256:deb-kernel")
	store.SaveIndex(idx)

	req := &ResolveRequest{Distro: "debian", Version: "6.1.0-25", Arch: "amd64"}
	loaded, err := store.Resolve(req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if loaded.ID != "sha256:deb-kernel" {
		t.Errorf("expected sha256:deb-kernel, got %s", loaded.ID)
	}
}

func TestStore_List(t *testing.T) {
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Images:     filepath.Join(tmp, "images"),
		Kernels:    filepath.Join(tmp, "kernels"),
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
		Tmp:        filepath.Join(tmp, "tmp"),
	}
	paths.EnsureAll()

	store := NewStore(paths)

	for i, id := range []string{"sha256:k1", "sha256:k2"} {
		k := &Kernel{
			ID:      id,
			Distro:  "debian",
			Version: "6.1.0",
			Arch:    "amd64",
			Created: time.Now().Add(time.Duration(i) * time.Hour),
		}
		store.Save(k)
		idx, _ := store.LoadIndex()
		idx.Add("debian:6.1.0:"+id, id)
		store.SaveIndex(idx)
	}

	kernels, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(kernels) < 2 {
		t.Errorf("expected at least 2 kernels, got %d", len(kernels))
	}
}

func TestStore_HasKernelImage(t *testing.T) {
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Images:     filepath.Join(tmp, "images"),
		Kernels:    filepath.Join(tmp, "kernels"),
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
		Tmp:        filepath.Join(tmp, "tmp"),
	}
	paths.EnsureAll()

	store := NewStore(paths)

	if store.HasKernelImage("sha256:nonexistent") {
		t.Error("should return false for nonexistent kernel")
	}

	kernelPath := paths.KernelImagePath("sha256:test")
	os.MkdirAll(filepath.Dir(kernelPath), 0o755)
	os.WriteFile(kernelPath, []byte("fake kernel"), 0o644)

	if !store.HasKernelImage("sha256:test") {
		t.Error("should return true for existing kernel")
	}
}

func TestStore_Remove(t *testing.T) {
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Images:     filepath.Join(tmp, "images"),
		Kernels:    filepath.Join(tmp, "kernels"),
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
		Tmp:        filepath.Join(tmp, "tmp"),
	}
	paths.EnsureAll()

	store := NewStore(paths)

	k := &Kernel{ID: "sha256:toremove", Created: time.Now()}
	store.Save(k)
	idx, _ := store.LoadIndex()
	idx.Add("test:toremove", "sha256:toremove")
	store.SaveIndex(idx)

	if err := store.Remove("sha256:toremove"); err != nil {
		t.Fatalf("remove: %v", err)
	}

	_, err := store.Get("sha256:toremove")
	if err == nil {
		t.Error("expected error after removal")
	}
}
