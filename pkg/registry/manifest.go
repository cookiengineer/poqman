package registry

import (
	"encoding/json"
)

type Manifest struct {
	SchemaVersion int              `json:"schemaVersion"`
	MediaType     string           `json:"mediaType"`
	Config        ManifestBlobRef  `json:"config"`
	Layers        []ManifestBlobRef `json:"layers"`
}

type ManifestBlobRef struct {
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
}

type ManifestList struct {
	SchemaVersion int                  `json:"schemaVersion"`
	MediaType     string               `json:"mediaType"`
	Manifests     []ManifestListEntry  `json:"manifests"`
}

type ManifestListEntry struct {
	MediaType string   `json:"mediaType"`
	Size      int64    `json:"size"`
	Digest    string   `json:"digest"`
	Platform  *Platform `json:"platform,omitempty"`
}

type OCIImageConfig struct {
	Created      string          `json:"created,omitempty"`
	Architecture string          `json:"architecture"`
	OS           string          `json:"os"`
	Variant      string          `json:"variant,omitempty"`
	Config       OCIConfigBlock  `json:"config"`
	RootFS       OCIRootFS       `json:"rootfs"`
	History      []OCIHistory    `json:"history,omitempty"`
}

type OCIConfigBlock struct {
	User         string              `json:"User,omitempty"`
	Env          []string            `json:"Env,omitempty"`
	Cmd          []string            `json:"Cmd,omitempty"`
	Entrypoint   []string            `json:"Entrypoint,omitempty"`
	WorkingDir   string              `json:"WorkingDir,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	Volumes      map[string]struct{} `json:"Volumes,omitempty"`
	Labels       map[string]string   `json:"Labels,omitempty"`
	StopSignal   string              `json:"StopSignal,omitempty"`
	Shell        []string            `json:"Shell,omitempty"`
}

type OCIRootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

type OCIHistory struct {
	Created    string `json:"created,omitempty"`
	CreatedBy  string `json:"created_by,omitempty"`
	EmptyLayer bool   `json:"empty_layer,omitempty"`
}

func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.SchemaVersion != 2 {
		return nil, nil
	}
	return &m, nil
}

func ParseManifestList(data []byte) (*ManifestList, error) {
	var ml ManifestList
	if err := json.Unmarshal(data, &ml); err != nil {
		return nil, err
	}
	if ml.SchemaVersion != 2 || len(ml.Manifests) == 0 {
		return nil, nil
	}
	return &ml, nil
}

func ParseImageConfig(data []byte) (*OCIImageConfig, error) {
	var cfg OCIImageConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func IsManifestList(mediaType string) bool {
	switch mediaType {
	case "application/vnd.oci.image.index.v1+json":
		return true
	case "application/vnd.docker.distribution.manifest.list.v2+json":
		return true
	}
	return false
}
