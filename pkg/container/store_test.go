package container

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cookiengineer/poqman/pkg/storage"
)

func TestStore_CreateAndLoad(t *testing.T) {
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

	c := &Container{
		ID:        "test-container-1",
		ImageID:   "sha256:abc123",
		ImageName: "alpine:latest",
		Command:   []string{"/bin/sh"},
		CreatedAt: time.Now(),
	}

	if err := store.Create(c); err != nil {
		t.Fatalf("create: %v", err)
	}

	if c.Status != StatusCreated {
		t.Errorf("expected StatusCreated, got %s", c.Status)
	}

	loaded, err := store.Load("test-container-1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ID != "test-container-1" {
		t.Errorf("expected test-container-1, got %s", loaded.ID)
	}
	if loaded.ImageID != "sha256:abc123" {
		t.Errorf("expected sha256:abc123, got %s", loaded.ImageID)
	}
	if loaded.Status != StatusCreated {
		t.Errorf("expected StatusCreated, got %s", loaded.Status)
	}
}

func TestStore_SaveAndUpdateState(t *testing.T) {
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

	c := &Container{
		ID:        "test-update",
		ImageID:   "sha256:def456",
		CreatedAt: time.Now(),
	}
	store.Create(c)

	c.Status = StatusRunning
	c.PID = 12345
	c.StartedAt = time.Now()
	store.Save(c)

	loaded, err := store.Load("test-update")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Status != StatusRunning {
		t.Errorf("expected StatusRunning, got %s", loaded.Status)
	}
	if loaded.PID != 12345 {
		t.Errorf("expected PID 12345, got %d", loaded.PID)
	}

	c.Status = StatusStopped
	c.FinishedAt = time.Now()
	store.Save(c)

	loaded, err = store.Load("test-update")
	if err != nil {
		t.Fatalf("load after stop: %v", err)
	}
	if loaded.Status != StatusStopped {
		t.Errorf("expected StatusStopped, got %s", loaded.Status)
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

	for _, id := range []string{"c1", "c2", "c3"} {
		c := &Container{
			ID:        id,
			CreatedAt: time.Now(),
			Status:    StatusCreated,
		}
		store.Create(c)
	}

	containers, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(containers) != 3 {
		t.Errorf("expected 3 containers, got %d", len(containers))
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

	c := &Container{ID: "to-remove", CreatedAt: time.Now()}
	store.Create(c)

	if err := store.Remove("to-remove"); err != nil {
		t.Fatalf("remove: %v", err)
	}

	_, err := store.Load("to-remove")
	if err == nil {
		t.Error("expected error after removal")
	}

	_, err = os.Stat(paths.ContainerPath("to-remove"))
	if !os.IsNotExist(err) {
		t.Error("expected container directory to be removed")
	}
}

func TestStore_LoadNotFound(t *testing.T) {
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
	_, err := store.Load("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}
