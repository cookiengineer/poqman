package container

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/cookiengineer/poqman/pkg/storage"
)

type Store struct {
	paths *storage.Paths
}

func NewStore(paths *storage.Paths) *Store {
	return &Store{paths: paths}
}

func (s *Store) Create(container *Container) error {
	container.Status = StatusCreated
	container.CreatedAt = time.Now()

	dir := s.paths.ContainerPath(container.ID)
	if err := os.MkdirAll(dir, storage.DefaultPerms); err != nil {
		return fmt.Errorf("create container dir: %w", err)
	}

	configPath := s.paths.ContainerConfigPath(container.ID)
	configData, err := json.MarshalIndent(container, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal container config: %w", err)
	}
	if err := os.WriteFile(configPath, configData, storage.FilePerms); err != nil {
		return fmt.Errorf("write container config: %w", err)
	}

	return s.saveState(container)
}

func (s *Store) Load(id string) (*Container, error) {
	configPath := s.paths.ContainerConfigPath(id)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("container %q not found", id)
		}
		return nil, fmt.Errorf("read container config: %w", err)
	}
	var container Container
	if err := json.Unmarshal(data, &container); err != nil {
		return nil, fmt.Errorf("parse container config: %w", err)
	}

	statePath := s.paths.ContainerStatePath(id)
	stateData, err := os.ReadFile(statePath)
	if err == nil {
		var state struct {
			Status     ContainerStatus `json:"status"`
			PID        int             `json:"pid"`
			IP         string          `json:"ip"`
			StartedAt  time.Time       `json:"startedAt"`
			FinishedAt time.Time       `json:"finishedAt"`
			ExitCode   int             `json:"exitCode"`
		}
		json.Unmarshal(stateData, &state)
		container.Status = state.Status
		container.PID = state.PID
		container.IP = state.IP
		container.StartedAt = state.StartedAt
		container.FinishedAt = state.FinishedAt
		container.ExitCode = state.ExitCode
	}

	return &container, nil
}

func (s *Store) Save(container *Container) error {
	configPath := s.paths.ContainerConfigPath(container.ID)
	configData, err := json.MarshalIndent(container, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal container config: %w", err)
	}
	if err := os.WriteFile(configPath, configData, storage.FilePerms); err != nil {
		return fmt.Errorf("write container config: %w", err)
	}
	return s.saveState(container)
}

func (s *Store) Remove(id string) error {
	containerDir := s.paths.ContainerPath(id)
	return os.RemoveAll(containerDir)
}

func (s *Store) List() ([]*Container, error) {
	entries, err := os.ReadDir(s.paths.Containers)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read containers dir: %w", err)
	}
	var containers []*Container
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		container, err := s.Load(entry.Name())
		if err != nil {
			continue
		}
		containers = append(containers, container)
	}
	sort.Slice(containers, func(i, j int) bool {
		return containers[i].CreatedAt.After(containers[j].CreatedAt)
	})
	return containers, nil
}

func (s *Store) saveState(container *Container) error {
	statePath := s.paths.ContainerStatePath(container.ID)
	state := map[string]any{
		"status":     container.Status,
		"pid":        container.PID,
		"ip":         container.IP,
		"startedAt":  container.StartedAt,
		"finishedAt": container.FinishedAt,
		"exitCode":   container.ExitCode,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal container state: %w", err)
	}
	return os.WriteFile(statePath, data, storage.FilePerms)
}
