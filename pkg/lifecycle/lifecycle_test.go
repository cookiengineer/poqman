package lifecycle

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cookiengineer/poqman/pkg/container"
	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func setupPaths(t *testing.T) *storage.Paths {
	t.Helper()
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
	return paths
}

func TestContainerLifecycle_CreateToStopped(t *testing.T) {
	paths := setupPaths(t)
	store := container.NewStore(paths)

	c := &container.Container{
		ID:        "lifecycle-1",
		ImageID:   "sha256:test-img",
		ImageName: "test:latest",
		Command:   []string{"/bin/sh"},
		CreatedAt: time.Now(),
	}

	if err := store.Create(c); err != nil {
		t.Fatalf("create: %v", err)
	}

	loaded, err := store.Load("lifecycle-1")
	if err != nil {
		t.Fatalf("load after create: %v", err)
	}
	if loaded.Status != container.StatusCreated {
		t.Errorf("expected Created after Create(), got %s", loaded.Status)
	}
	if loaded.ID != "lifecycle-1" {
		t.Errorf("expected lifecycle-1, got %s", loaded.ID)
	}

	c.Status = container.StatusRunning
	c.PID = 12345
	c.StartedAt = time.Now()
	if err := store.Save(c); err != nil {
		t.Fatalf("save running: %v", err)
	}

	c.Status = container.StatusStopped
	c.FinishedAt = time.Now()
	c.ExitCode = 0
	if err := store.Save(c); err != nil {
		t.Fatalf("save stopped: %v", err)
	}

	loaded, _ = store.Load("lifecycle-1")
	if loaded.Status != container.StatusStopped {
		t.Errorf("expected Stopped, got %s", loaded.Status)
	}
	if loaded.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", loaded.ExitCode)
	}
	if loaded.FinishedAt.IsZero() {
		t.Error("expected non-zero FinishedAt")
	}
}

func TestContainerLifecycle_CreateToFailed(t *testing.T) {
	paths := setupPaths(t)
	store := container.NewStore(paths)

	c := &container.Container{
		ID:        "failed-1",
		ImageID:   "sha256:broken",
		ImageName: "broken:latest",
		CreatedAt: time.Now(),
	}
	store.Create(c)

	c.Status = container.StatusRunning
	c.PID = 12346
	store.Save(c)

	c.Status = container.StatusFailed
	c.ExitCode = 255
	c.FinishedAt = time.Now()
	store.Save(c)

	loaded, _ := store.Load("failed-1")
	if loaded.Status != container.StatusFailed {
		t.Errorf("expected Failed, got %s", loaded.Status)
	}
	if loaded.ExitCode != 255 {
		t.Errorf("expected exit 255, got %d", loaded.ExitCode)
	}
}

func TestContainerLifecycle_ListMultipleContainers(t *testing.T) {
	paths := setupPaths(t)
	store := container.NewStore(paths)

	statuses := []container.ContainerStatus{
		container.StatusCreated,
		container.StatusRunning,
		container.StatusStopped,
		container.StatusFailed,
	}
	for _, status := range statuses {
		c := &container.Container{
			ID:        container.GenerateID(),
			ImageID:   "sha256:img",
			CreatedAt: time.Now(),
		}
		store.Create(c)
		c.Status = status
		store.Save(c)
	}

	containers, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(containers) != 4 {
		t.Fatalf("expected 4 containers, got %d", len(containers))
	}

	found := make(map[container.ContainerStatus]int)
	for _, c := range containers {
		found[c.Status]++
	}
	if found[container.StatusCreated] != 1 {
		t.Errorf("expected 1 Created, got %d", found[container.StatusCreated])
	}
	if found[container.StatusRunning] != 1 {
		t.Errorf("expected 1 Running, got %d", found[container.StatusRunning])
	}
	if found[container.StatusStopped] != 1 {
		t.Errorf("expected 1 Stopped, got %d", found[container.StatusStopped])
	}
	if found[container.StatusFailed] != 1 {
		t.Errorf("expected 1 Failed, got %d", found[container.StatusFailed])
	}
}

func TestContainerLifecycle_RemoveStoppedContainer(t *testing.T) {
	paths := setupPaths(t)
	store := container.NewStore(paths)

	c := &container.Container{
		ID:        "to-remove",
		Status:    container.StatusStopped,
		CreatedAt: time.Now(),
	}
	store.Create(c)

	if err := store.Remove("to-remove"); err != nil {
		t.Fatalf("remove: %v", err)
	}

	_, err := store.Load("to-remove")
	if err == nil {
		t.Error("expected error loading removed container")
	}

	_, err = os.Stat(paths.ContainerPath("to-remove"))
	if !os.IsNotExist(err) {
		t.Error("expected container directory to be removed")
	}
}

func TestImageLifecycle_SaveAndTag(t *testing.T) {
	paths := setupPaths(t)
	store := image.NewStore(paths)

	img := &image.Image{
		ID:       "sha256:lifecycle-img",
		RepoTags: []string{"lifecycle:test", "lifecycle:latest"},
		Arch:     "amd64",
		Config: image.ImageConfig{
			Cmd: []string{"/bin/sh"},
			Env: []string{"PATH=/usr/bin"},
		},
		Layers: []image.Layer{
			{Digest: "sha256:layer1", Size: 1024, MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip"},
		},
		Created: time.Now(),
		Size:    1024,
	}

	if err := store.Save(img); err != nil {
		t.Fatalf("save: %v", err)
	}

	idx, _ := store.LoadIndex()
	idx.Add("docker.io/library/lifecycle:test", "sha256:lifecycle-img")
	idx.Add("docker.io/library/lifecycle:latest", "sha256:lifecycle-img")
	store.SaveIndex(idx)

	ref, _ := image.ParseImageRef("lifecycle:test")
	resolved, err := store.Resolve(ref)
	if err != nil {
		t.Fatalf("resolve by tag: %v", err)
	}
	if resolved.ID != "sha256:lifecycle-img" {
		t.Errorf("expected sha256:lifecycle-img, got %s", resolved.ID)
	}

	loaded, err := store.Get("sha256:lifecycle-img")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(loaded.Config.Cmd) != 1 || loaded.Config.Cmd[0] != "/bin/sh" {
		t.Errorf("unexpected Cmd: %v", loaded.Config.Cmd)
	}
}

func TestImageLifecycle_UntagAndRemove(t *testing.T) {
	paths := setupPaths(t)
	store := image.NewStore(paths)

	img := &image.Image{
		ID:       "sha256:untag-me",
		RepoTags: []string{"untag:latest"},
		Created:  time.Now(),
	}
	store.Save(img)
	idx, _ := store.LoadIndex()
	idx.Add("untag:latest", "sha256:untag-me")
	store.SaveIndex(idx)

	if err := store.Remove("sha256:untag-me"); err != nil {
		t.Fatalf("remove: %v", err)
	}

	_, err := store.Get("sha256:untag-me")
	if err == nil {
		t.Error("expected error loading removed image")
	}

	_, err = os.Stat(paths.ImagePath("sha256:untag-me"))
	if !os.IsNotExist(err) {
		t.Error("expected image directory to be removed")
	}
}

func TestMixedLifecycle_ImageAndContainers(t *testing.T) {
	paths := setupPaths(t)

	imgStore := image.NewStore(paths)
	img := &image.Image{
		ID:       "sha256:shared-img",
		RepoTags: []string{"shared:latest"},
		Arch:     "amd64",
		Created:  time.Now(),
	}
	imgStore.Save(img)
	idx, _ := imgStore.LoadIndex()
	idx.Add("shared:latest", "sha256:shared-img")
	imgStore.SaveIndex(idx)

	containerStore := container.NewStore(paths)
	for i := 0; i < 3; i++ {
		c := &container.Container{
			ID:        container.GenerateID(),
			ImageID:   "sha256:shared-img",
			ImageName: "shared:latest",
			Status:    container.StatusCreated,
			CreatedAt: time.Now(),
		}
		containerStore.Create(c)
	}

	containers, _ := containerStore.List()
	if len(containers) != 3 {
		t.Fatalf("expected 3 containers, got %d", len(containers))
	}
	for _, c := range containers {
		if c.ImageID != "sha256:shared-img" {
			t.Errorf("expected image sha256:shared-img, got %s", c.ImageID)
		}
	}
}

func TestMixedLifecycle_RemoveContainerPreservesImage(t *testing.T) {
	paths := setupPaths(t)

	imgStore := image.NewStore(paths)
	img := &image.Image{
		ID:       "sha256:persistent",
		RepoTags: []string{"persist:latest"},
		Created:  time.Now(),
	}
	imgStore.Save(img)
	idx, _ := imgStore.LoadIndex()
	idx.Add("persist:latest", "sha256:persistent")
	imgStore.SaveIndex(idx)

	containerStore := container.NewStore(paths)
	c := &container.Container{
		ID:        "temp-container",
		ImageID:   "sha256:persistent",
		Status:    container.StatusStopped,
		CreatedAt: time.Now(),
	}
	containerStore.Create(c)

	containerStore.Remove("temp-container")
	_, err := containerStore.Load("temp-container")
	if err == nil {
		t.Error("expected container to be gone")
	}

	loaded, err := imgStore.Get("sha256:persistent")
	if err != nil {
		t.Fatalf("image should still exist after container removal: %v", err)
	}
	if loaded.ID != "sha256:persistent" {
		t.Errorf("unexpected image ID: %s", loaded.ID)
	}
}

func TestImageLifecycle_MultipleTagsSameImage(t *testing.T) {
	paths := setupPaths(t)
	store := image.NewStore(paths)

	img := &image.Image{
		ID:       "sha256:multi-tag",
		RepoTags: []string{"app:v1", "app:latest", "app:stable"},
		Created:  time.Now(),
	}
	store.Save(img)
	idx, _ := store.LoadIndex()
	idx.Add("docker.io/library/app:v1", "sha256:multi-tag")
	idx.Add("docker.io/library/app:latest", "sha256:multi-tag")
	idx.Add("docker.io/library/app:stable", "sha256:multi-tag")
	store.SaveIndex(idx)

	tagsToTest := []string{"app:v1", "app:latest"}
	for _, raw := range tagsToTest {
		ref, _ := image.ParseImageRef(raw)
		resolved, err := store.Resolve(ref)
		if err != nil {
			t.Errorf("resolve %q: %v", raw, err)
			continue
		}
		if resolved.ID != "sha256:multi-tag" {
			t.Errorf("resolved %q → %s, want sha256:multi-tag", raw, resolved.ID)
		}
	}

	loaded, _ := store.Get("sha256:multi-tag")
	if len(loaded.RepoTags) != 3 {
		t.Errorf("expected 3 repoTags, got %d: %v", len(loaded.RepoTags), loaded.RepoTags)
	}
}

func TestContainerLifecycle_StateTransitions(t *testing.T) {
	paths := setupPaths(t)
	store := container.NewStore(paths)

	validTransitions := []struct {
		from container.ContainerStatus
		to   container.ContainerStatus
	}{
		{container.StatusCreated, container.StatusRunning},
		{container.StatusRunning, container.StatusStopped},
		{container.StatusRunning, container.StatusFailed},
		{container.StatusStopped, container.StatusRunning},
	}

	for i, tr := range validTransitions {
		id := container.GenerateID()
		c := &container.Container{
			ID:        id,
			Status:    tr.from,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Minute),
		}
		store.Create(c)

		c.Status = tr.to
		if tr.to == container.StatusRunning {
			c.PID = 10000 + i
			c.StartedAt = time.Now()
		}
		if tr.to == container.StatusStopped || tr.to == container.StatusFailed {
			c.FinishedAt = time.Now()
		}
		store.Save(c)

		loaded, err := store.Load(id)
		if err != nil {
			t.Errorf("transition %s→%s: load: %v", tr.from, tr.to, err)
			continue
		}
		if loaded.Status != tr.to {
			t.Errorf("transition %s→%s: got %s", tr.from, tr.to, loaded.Status)
		}
	}
}

func TestContainerLifecycle_NamePreserved(t *testing.T) {
	paths := setupPaths(t)
	store := container.NewStore(paths)

	c := &container.Container{
		ID:        "named-container",
		ImageID:   "sha256:img",
		ImageName: "test:latest",
		Name:      "my-webserver",
		Command:   []string{"nginx"},
		CreatedAt: time.Now(),
	}
	store.Create(c)

	c.Status = container.StatusRunning
	c.PID = 12347
	store.Save(c)

	loaded, _ := store.Load("named-container")
	if loaded.Name != "my-webserver" {
		t.Errorf("expected name my-webserver, got %s", loaded.Name)
	}
	if len(loaded.Command) != 1 || loaded.Command[0] != "nginx" {
		t.Errorf("command should survive state transitions: %v", loaded.Command)
	}
}

func TestContainerLifecycle_PortMappingSurvivesTransitions(t *testing.T) {
	paths := setupPaths(t)
	store := container.NewStore(paths)

	c := &container.Container{
		ID:     "port-container",
		Status: container.StatusCreated,
		Ports: []container.PortMapping{
			{HostPort: 8080, GuestPort: 80, Protocol: "tcp"},
		},
		CreatedAt: time.Now(),
	}
	store.Create(c)

	c.Status = container.StatusRunning
	c.PID = 1
	c.IP = "10.88.0.2"
	store.Save(c)

	c.Status = container.StatusStopped
	store.Save(c)

	loaded, _ := store.Load("port-container")
	if len(loaded.Ports) != 1 {
		t.Errorf("expected 1 port mapping after transitions, got %d", len(loaded.Ports))
	}
	if loaded.Ports[0].HostPort != 8080 {
		t.Errorf("expected hostPort 8080, got %d", loaded.Ports[0].HostPort)
	}
}

func TestContainerLifecycle_VolumeMountSurvivesTransitions(t *testing.T) {
	paths := setupPaths(t)
	store := container.NewStore(paths)

	c := &container.Container{
		ID:     "volume-container",
		Status: container.StatusCreated,
		Volumes: []container.VolumeMount{
			{Source: "/host/data", Target: "/data", ReadOnly: false},
			{Source: "/host/config", Target: "/config", ReadOnly: true},
		},
		CreatedAt: time.Now(),
	}
	store.Create(c)

	c.Status = container.StatusRunning
	store.Save(c)

	loaded, _ := store.Load("volume-container")
	if len(loaded.Volumes) != 2 {
		t.Errorf("expected 2 volumes, got %d", len(loaded.Volumes))
	}
	if loaded.Volumes[1].ReadOnly != true {
		t.Error("expected second volume to be ReadOnly")
	}
}

func TestImageLifecycle_FullConfigRoundTrip(t *testing.T) {
	paths := setupPaths(t)
	store := image.NewStore(paths)

	img := &image.Image{
		ID:       "sha256:full-config",
		RepoTags: []string{"full:config"},
		Arch:     "arm64",
		Config: image.ImageConfig{
			User:         "1000",
			Env:          []string{"PATH=/usr/bin", "HOME=/home/app"},
			Cmd:          []string{"node", "server.js"},
			Entrypoint:   []string{"/entrypoint.sh"},
			Workdir:      "/app",
			ExposedPorts: map[string]struct{}{"3000/tcp": {}, "9229/tcp": {}},
			Volumes:      map[string]struct{}{"/data": {}, "/tmp": {}},
			Labels:       map[string]string{"com.example.version": "1.0.0"},
			StopSignal:   "SIGTERM",
			Shell:        []string{"/bin/sh", "-c"},
		},
		Layers: []image.Layer{
			{Digest: "sha256:l1", Size: 5000},
			{Digest: "sha256:l2", Size: 3000},
		},
		KernelRef: "debian:6.1.0-25",
		Created:   time.Now(),
		Size:      8000,
	}
	store.Save(img)
	idx, _ := store.LoadIndex()
	idx.Add("full:config", "sha256:full-config")
	store.SaveIndex(idx)

	loaded, err := store.Get("sha256:full-config")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if loaded.Arch != "arm64" {
		t.Errorf("arch: expected arm64, got %s", loaded.Arch)
	}
	if loaded.Config.User != "1000" {
		t.Errorf("user: expected 1000, got %s", loaded.Config.User)
	}
	if loaded.Config.Workdir != "/app" {
		t.Errorf("workdir: expected /app, got %s", loaded.Config.Workdir)
	}
	if len(loaded.Config.Env) != 2 {
		t.Errorf("env: expected 2, got %d", len(loaded.Config.Env))
	}
	if loaded.Config.Entrypoint[0] != "/entrypoint.sh" {
		t.Errorf("entrypoint: expected /entrypoint.sh, got %s", loaded.Config.Entrypoint[0])
	}
	if loaded.KernelRef != "debian:6.1.0-25" {
		t.Errorf("kernel: expected debian:6.1.0-25, got %s", loaded.KernelRef)
	}
	if loaded.Size != 8000 {
		t.Errorf("size: expected 8000, got %d", loaded.Size)
	}
	if len(loaded.Layers) != 2 {
		t.Errorf("layers: expected 2, got %d", len(loaded.Layers))
	}
	if loaded.Layers[0].Size != 5000 {
		t.Errorf("layer0 size: expected 5000, got %d", loaded.Layers[0].Size)
	}
}

func TestContainerLifecycle_GenerateIDUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 500; i++ {
		id := container.GenerateID()
		if seen[id] {
			t.Errorf("duplicate ID generated: %s", id)
		}
		seen[id] = true
		if len(id) != 12 {
			t.Errorf("unexpected ID length: %d", len(id))
		}
	}
}
