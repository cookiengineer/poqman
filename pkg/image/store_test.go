package image

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cookiengineer/poqman/pkg/storage"
)

func TestStore_ConcurrentIndexAccess(t *testing.T) {
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

	img := &Image{
		ID:       "sha256:concurrent",
		RepoTags: []string{"test:concurrent"},
		Created:  time.Now(),
	}
	store.Save(img)
	idx, _ := store.LoadIndex()
	idx.Add("test:concurrent", "sha256:concurrent")
	store.SaveIndex(idx)

	var wg sync.WaitGroup
	errors := make(chan error, 20)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			idx, err := store.LoadIndex()
			if err != nil {
				errors <- err
				return
			}
			if _, ok := idx.Lookup("test:concurrent"); !ok {
				errors <- nil
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			idx, _ := store.LoadIndex()
			idx.Add("test:concurrent", "sha256:concurrent")
			if err := store.SaveIndex(idx); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("concurrent access error: %v", err)
		}
	}

	finalIdx, err := store.LoadIndex()
	if err != nil {
		t.Fatalf("load final index: %v", err)
	}
	if _, ok := finalIdx.Lookup("test:concurrent"); !ok {
		t.Error("index entry missing after concurrent access")
	}
}

func TestStore_ConcurrentSaveAndRead(t *testing.T) {
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

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			img := &Image{
				ID:       "sha256:conc-" + string(rune('0'+id)),
				RepoTags: []string{"test:conc-" + string(rune('0'+id))},
				Created:  time.Now(),
			}
			store.Save(img)
			idx, _ := store.LoadIndex()
			idx.Add("test:conc-"+string(rune('0'+id)), "sha256:conc-"+string(rune('0'+id)))
			store.SaveIndex(idx)
		}(i)
	}

	wg.Wait()

	images, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(images) < 1 {
		t.Error("expected at least one image after concurrent saves")
	}
}

func TestImageIndex_ConcurrentAddRemove(t *testing.T) {
	idx := NewImageIndex()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "test:" + string(rune('a'+n%26))
			idx.Add(key, "id")
		}(i)
	}

	wg.Wait()

	if len(idx.Images) < 1 {
		t.Error("expected entries after concurrent adds")
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

	img := &Image{
		ID:       "sha256:test123",
		RepoTags: []string{"docker.io/library/test:latest"},
		Arch:     "amd64",
		Config: ImageConfig{
			Env: []string{"PATH=/usr/bin"},
			Cmd: []string{"/bin/sh"},
		},
		Layers: []Layer{
			{Digest: "sha256:layer1", Size: 1024, MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip"},
		},
		Created: time.Now(),
		Size:    1024,
	}

	if err := store.Save(img); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Get("sha256:test123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded.ID != img.ID {
		t.Errorf("expected ID %s, got %s", img.ID, loaded.ID)
	}
	if len(loaded.RepoTags) != 1 || loaded.RepoTags[0] != "docker.io/library/test:latest" {
		t.Errorf("unexpected repoTags: %v", loaded.RepoTags)
	}
	if loaded.Config.Cmd[0] != "/bin/sh" {
		t.Errorf("unexpected Cmd: %v", loaded.Config.Cmd)
	}
}

func TestStore_ResolveAndIndex(t *testing.T) {
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

	img := &Image{
		ID:       "sha256:abcdef",
		RepoTags: []string{"docker.io/library/alpine:latest"},
		Arch:     "amd64",
		Created:  time.Now(),
	}
	store.Save(img)

	idx, _ := store.LoadIndex()
	idx.Add("docker.io/library/alpine:latest", "sha256:abcdef")
	store.SaveIndex(idx)

	ref, _ := ParseImageRef("alpine:latest")
	resolved, err := store.Resolve(ref)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.ID != "sha256:abcdef" {
		t.Errorf("expected sha256:abcdef, got %s", resolved.ID)
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

	for i, id := range []string{"sha256:aaa111", "sha256:bbb222"} {
		img := &Image{
			ID:       id,
			RepoTags: []string{"test:tag"},
			Created:  time.Now().Add(time.Duration(i) * time.Hour),
		}
		store.Save(img)
		idx, _ := store.LoadIndex()
		idx.Add("test:"+id[:8], id)
		store.SaveIndex(idx)
	}

	images, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(images) < 2 {
		t.Errorf("expected at least 2 images, got %d", len(images))
	}
}

func TestStore_GetNotFound(t *testing.T) {
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
	_, err := store.Get("sha256:nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent image")
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

	img := &Image{ID: "sha256:toremove", RepoTags: []string{"test:remove"}, Created: time.Now()}
	store.Save(img)
	idx, _ := store.LoadIndex()
	idx.Add("test:remove", "sha256:toremove")
	store.SaveIndex(idx)

	if err := store.Remove("sha256:toremove"); err != nil {
		t.Fatalf("remove: %v", err)
	}

	_, err := store.Get("sha256:toremove")
	if err == nil {
		t.Error("expected error after removal")
	}

	_, err = os.Stat(paths.ImagePath("sha256:toremove"))
	if !os.IsNotExist(err) {
		t.Error("expected image directory to be removed")
	}
}
