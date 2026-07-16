package kernel

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/cookiengineer/poqman/pkg/storage"
)

type Store struct {
	paths *storage.Paths
}

func NewStore(paths *storage.Paths) *Store {
	return &Store{paths: paths}
}

func (s *Store) LoadIndex() (*KernelIndex, error) {
	path := s.paths.KernelIndexPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewKernelIndex(), nil
		}
		return nil, fmt.Errorf("read kernel index: %w", err)
	}
	idx := NewKernelIndex()
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, fmt.Errorf("parse kernel index: %w", err)
	}
	return idx, nil
}

func (s *Store) SaveIndex(idx *KernelIndex) error {
	path := s.paths.KernelIndexPath()
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal kernel index: %w", err)
	}
	return os.WriteFile(path, data, storage.FilePerms)
}

func (s *Store) Get(id string) (*Kernel, error) {
	configPath := s.paths.KernelConfigPath(id)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("kernel %q not found", id)
		}
		return nil, fmt.Errorf("read kernel config: %w", err)
	}
	var k Kernel
	if err := json.Unmarshal(data, &k); err != nil {
		return nil, fmt.Errorf("parse kernel config: %w", err)
	}
	return &k, nil
}

func (s *Store) Save(kernel *Kernel) error {
	dir := s.paths.KernelPath(kernel.ID)
	if err := os.MkdirAll(dir, storage.DefaultPerms); err != nil {
		return fmt.Errorf("create kernel dir: %w", err)
	}

	configPath := s.paths.KernelConfigPath(kernel.ID)
	data, err := json.MarshalIndent(kernel, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal kernel config: %w", err)
	}
	return os.WriteFile(configPath, data, storage.FilePerms)
}

func (s *Store) Remove(id string) error {
	idx, err := s.LoadIndex()
	if err != nil {
		return err
	}
	for name, kernelID := range idx.Kernels {
		if kernelID == id {
			delete(idx.Kernels, name)
		}
	}
	s.SaveIndex(idx)
	return os.RemoveAll(s.paths.KernelPath(id))
}

func (s *Store) HasKernelImage(id string) bool {
	imagePath := s.paths.KernelImagePath(id)
	_, err := os.Stat(imagePath)
	return err == nil
}

func (s *Store) Resolve(req *ResolveRequest) (*Kernel, error) {
	idx, err := s.LoadIndex()
	if err != nil {
		return nil, err
	}

	fullName := req.String()
	id, ok := idx.Lookup(fullName)
	if !ok {
		return nil, fmt.Errorf("kernel %q not found locally", fullName)
	}
	return s.Get(id)
}

func (s *Store) List() ([]*Kernel, error) {
	idx, err := s.LoadIndex()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var kernels []*Kernel
	for _, id := range idx.Kernels {
		if seen[id] {
			continue
		}
		seen[id] = true
		k, err := s.Get(id)
		if err != nil {
			continue
		}
		kernels = append(kernels, k)
	}
	sort.Slice(kernels, func(i, j int) bool {
		return kernels[i].Created.After(kernels[j].Created)
	})
	return kernels, nil
}
