package image

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/cookiengineer/poqman/pkg/storage"
)

type Store struct {
	paths *storage.Paths
}

func NewStore(paths *storage.Paths) *Store {
	return &Store{paths: paths}
}

func (s *Store) LoadIndex() (*ImageIndex, error) {
	path := s.paths.ImageIndexPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewImageIndex(), nil
		}
		return nil, fmt.Errorf("read image index: %w", err)
	}
	idx := NewImageIndex()
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, fmt.Errorf("parse image index: %w", err)
	}
	return idx, nil
}

func (s *Store) SaveIndex(idx *ImageIndex) error {
	path := s.paths.ImageIndexPath()
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal image index: %w", err)
	}
	if err := os.WriteFile(path, data, storage.FilePerms); err != nil {
		return fmt.Errorf("write image index: %w", err)
	}
	return nil
}

func (s *Store) Get(id string) (*Image, error) {
	configPath := s.paths.ImageConfigPath(id)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("image %q not found", id)
		}
		return nil, fmt.Errorf("read image config: %w", err)
	}
	var img Image
	if err := json.Unmarshal(data, &img); err != nil {
		return nil, fmt.Errorf("parse image config: %w", err)
	}
	return &img, nil
}

func (s *Store) Save(img *Image) error {
	configPath := s.paths.ImageConfigPath(img.ID)
	manifestPath := s.paths.ImageManifestPath(img.ID)
	layersDir := s.paths.ImageLayersDir(img.ID)
	kernelDir := s.paths.ImageKernelDir(img.ID)

	for _, dir := range []string{
		filepath.Dir(configPath),
		layersDir,
		kernelDir,
	} {
		if err := os.MkdirAll(dir, storage.DefaultPerms); err != nil {
			return fmt.Errorf("create image dir: %w", err)
		}
	}

	data, err := json.MarshalIndent(img, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal image config: %w", err)
	}
	if err := os.WriteFile(configPath, data, storage.FilePerms); err != nil {
		return fmt.Errorf("write image config: %w", err)
	}

	manifestData := map[string]any{
		"id":      img.ID,
		"created": img.Created,
		"size":    img.Size,
		"layers":  img.Layers,
	}
	manifestBytes, _ := json.MarshalIndent(manifestData, "", "  ")
	os.WriteFile(manifestPath, manifestBytes, storage.FilePerms)

	return nil
}

func (s *Store) Remove(id string) error {
	idx, err := s.LoadIndex()
	if err != nil {
		return err
	}
	for name, imageID := range idx.Images {
		if imageID == id {
			delete(idx.Images, name)
		}
	}
	if err := s.SaveIndex(idx); err != nil {
		return err
	}
	imageDir := s.paths.ImagePath(id)
	if err := os.RemoveAll(imageDir); err != nil {
		return fmt.Errorf("remove image dir: %w", err)
	}
	return nil
}

func (s *Store) Resolve(ref ImageRef) (*Image, error) {
	idx, err := s.LoadIndex()
	if err != nil {
		return nil, err
	}

	id, ok := idx.Lookup(ref.FullName())
	if !ok {
		taglessID, taglessOK := idx.Lookup(ref.Registry + "/" + ref.Repository + ":latest")
		if taglessOK {
			return s.Get(taglessID)
		}
		return nil, fmt.Errorf("image %q not found locally", ref.FullName())
	}
	return s.Get(id)
}

func (s *Store) List() ([]*Image, error) {
	idx, err := s.LoadIndex()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var images []*Image
	for _, id := range idx.Images {
		if seen[id] {
			continue
		}
		seen[id] = true
		img, err := s.Get(id)
		if err != nil {
			continue
		}
		images = append(images, img)
	}
	sort.Slice(images, func(i, j int) bool {
		return images[i].Created.After(images[j].Created)
	})
	return images, nil
}
