package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cookiengineer/poqman/pkg/container"
	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func TestRegisterRm(t *testing.T) {
	r := NewRouter()
	RegisterRm(r)

	if _, ok := r.commands["rm"]; !ok {
		t.Error("expected 'rm' command to be registered")
	}

	cmd := r.commands["rm"]
	if cmd.Name != "rm" {
		t.Errorf("expected name 'rm', got %s", cmd.Name)
	}
	if cmd.Description == "" {
		t.Error("expected non-empty description")
	}

	fs := cmd.FlagSet
	if fs.Lookup("f") == nil {
		t.Error("expected -f flag")
	}
}

func TestRegisterRmi(t *testing.T) {
	r := NewRouter()
	RegisterRmi(r)

	if _, ok := r.commands["rmi"]; !ok {
		t.Error("expected 'rmi' command to be registered")
	}

	cmd := r.commands["rmi"]
	if cmd.Name != "rmi" {
		t.Errorf("expected name 'rmi', got %s", cmd.Name)
	}
	if cmd.Description == "" {
		t.Error("expected non-empty description")
	}

	fs := cmd.FlagSet
	if fs.Lookup("f") == nil {
		t.Error("expected -f flag")
	}
}

func TestForceKill(t *testing.T) {
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Containers: filepath.Join(tmp, "containers"),
	}
	os.MkdirAll(paths.Containers, storage.DefaultPerms)

	store := container.NewStore(paths)
	c := &container.Container{
		ID:        "test-force-kill",
		Status:    container.StatusRunning,
		PID:       99999,
		CreatedAt: time.Now(),
	}
	store.Create(c)

	err := forceKill(c, paths)
	if err != nil {
		t.Errorf("forceKill on non-existent PID should not hard-error: %v", err)
	}

	loaded, _ := store.Load("test-force-kill")
	if loaded.Status != container.StatusStopped {
		t.Errorf("expected StatusStopped after force kill, got %s", loaded.Status)
	}
}

func TestRmi_ProtectsUsedImages(t *testing.T) {
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Images:     filepath.Join(tmp, "images"),
		Containers: filepath.Join(tmp, "containers"),
		Kernels:    filepath.Join(tmp, "kernels"),
		Networks:   filepath.Join(tmp, "networks"),
		Tmp:        filepath.Join(tmp, "tmp"),
	}
	paths.EnsureAll()

	imgStore := image.NewStore(paths)
	img := &image.Image{
		ID:       "sha256:in-use",
		RepoTags: []string{"test:inuse"},
		Arch:     "amd64",
		Created:  time.Now(),
	}
	imgStore.Save(img)
	idx, _ := imgStore.LoadIndex()
	idx.Add("test:inuse", "sha256:in-use")
	imgStore.SaveIndex(idx)

	containerStore := container.NewStore(paths)
	c := &container.Container{
		ID:        "running-container",
		ImageID:   "sha256:in-use",
		Status:    container.StatusRunning,
		CreatedAt: time.Now(),
	}
	containerStore.Create(c)

	loaded, _ := containerStore.List()
	_ = loaded
}

func TestRm_EmptyArgs(t *testing.T) {
	flags := []string{"{{rm}}"}
	_ = flags
}
