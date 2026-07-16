package image

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type Image struct {
	ID        string      `json:"id"`
	RepoTags  []string    `json:"repoTags"`
	Arch      string      `json:"arch"`
	Config    ImageConfig `json:"config"`
	Layers    []Layer     `json:"layers"`
	KernelRef string      `json:"kernelRef,omitempty"`
	Created   time.Time   `json:"created"`
	Size      int64       `json:"size"`
}

type ImageConfig struct {
	User         string              `json:"user,omitempty"`
	Env          []string            `json:"env,omitempty"`
	Cmd          []string            `json:"cmd,omitempty"`
	Entrypoint   []string            `json:"entrypoint,omitempty"`
	Workdir      string              `json:"workdir,omitempty"`
	ExposedPorts map[string]struct{} `json:"exposedPorts,omitempty"`
	Volumes      map[string]struct{} `json:"volumes,omitempty"`
	Labels       map[string]string   `json:"labels,omitempty"`
	StopSignal   string              `json:"stopSignal,omitempty"`
	Shell        []string            `json:"shell,omitempty"`
}

type Layer struct {
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	MediaType string `json:"mediaType"`
}

type ImageIndex struct {
	Images map[string]string `json:"images"`
}

func NewImageIndex() *ImageIndex {
	return &ImageIndex{Images: make(map[string]string)}
}

func (idx *ImageIndex) Add(name string, id string) {
	idx.Images[name] = id
}

func (idx *ImageIndex) Remove(name string) {
	delete(idx.Images, name)
}

func (idx *ImageIndex) Lookup(name string) (string, bool) {
	id, ok := idx.Images[name]
	return id, ok
}

func GenerateID(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%s", hex.EncodeToString(hash[:]))
}
