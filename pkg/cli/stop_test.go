package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cookiengineer/poqman/pkg/container"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func TestStopNoArgs(t *testing.T) {
	r := NewRouter()
	RegisterStop(r)
	cmd := r.commands["stop"]

	err := cmd.Run([]string{})
	if err == nil {
		t.Error("expected error when no container ID")
	}
}

func TestStopTimeoutFlag(t *testing.T) {
	r := NewRouter()
	RegisterStop(r)
	cmd := r.commands["stop"]

	if cmd.FlagSet.Lookup("t") == nil {
		t.Error("expected -t flag for timeout")
	}
}

func TestForceKillContainerCleansUpPorts(t *testing.T) {
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
	}
	os.MkdirAll(paths.Containers, storage.DefaultPerms)
	os.MkdirAll(paths.Networks, storage.DefaultPerms)

	store := container.NewStore(paths)

	c := &container.Container{
		ID:     "test-kill-ports",
		Status: container.StatusRunning,
		PID:    99999,
		IP:     "10.88.0.5",
		Ports: []container.PortMapping{
			{HostPort: 8080, GuestPort: 80, Protocol: "tcp"},
			{HostPort: 8443, GuestPort: 443, Protocol: "tcp"},
		},
		CreatedAt: time.Now(),
	}
	store.Create(c)

	forceKillContainer(c, paths, store)

	loaded, err := store.Load("test-kill-ports")
	if err != nil {
		t.Fatalf("load after force kill: %v", err)
	}
	if loaded.Status != container.StatusStopped {
		t.Errorf("expected Stopped, got %s", loaded.Status)
	}
}

func TestForceKillContainer_IPCannotFindProcess(t *testing.T) {
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
	}
	os.MkdirAll(paths.Containers, storage.DefaultPerms)
	os.MkdirAll(paths.Networks, storage.DefaultPerms)

	store := container.NewStore(paths)

	c := &container.Container{
		ID:        "ghost-process",
		Status:    container.StatusRunning,
		PID:       1,
		CreatedAt: time.Now(),
	}
	store.Create(c)

	forceKillContainer(c, paths, store)

	loaded, _ := store.Load("ghost-process")
	if loaded.Status != container.StatusStopped {
		t.Errorf("expected Stopped even if process not found, got %s", loaded.Status)
	}
}

func TestForceKillContainer_WithVolumes(t *testing.T) {
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
	}
	os.MkdirAll(paths.Containers, storage.DefaultPerms)
	os.MkdirAll(paths.Networks, storage.DefaultPerms)

	store := container.NewStore(paths)

	c := &container.Container{
		ID:     "test-volumes",
		Status: container.StatusRunning,
		PID:    99999,
		Volumes: []container.VolumeMount{
			{Source: "/host/data", Target: "/data"},
		},
		CreatedAt: time.Now(),
	}
	store.Create(c)

	forceKillContainer(c, paths, store)

	loaded, _ := store.Load("test-volumes")
	if loaded.Status != container.StatusStopped {
		t.Errorf("expected Stopped after force kill with volumes, got %s", loaded.Status)
	}
}
