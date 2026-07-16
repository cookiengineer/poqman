package registry

import (
	"encoding/json"
	"testing"
)

func TestParseManifest_DockerV2(t *testing.T) {
	data := `{
		"schemaVersion": 2,
		"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		"config": {
			"mediaType": "application/vnd.docker.container.image.v1+json",
			"size": 7023,
			"digest": "sha256:abc123"
		},
		"layers": [
			{
				"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
				"size": 32654,
				"digest": "sha256:layer1"
			}
		]
	}`

	manifest, err := ParseManifest([]byte(data))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if manifest.SchemaVersion != 2 {
		t.Errorf("expected schema 2, got %d", manifest.SchemaVersion)
	}
	if manifest.Config.Digest != "sha256:abc123" {
		t.Errorf("expected sha256:abc123, got %s", manifest.Config.Digest)
	}
	if len(manifest.Layers) != 1 {
		t.Errorf("expected 1 layer, got %d", len(manifest.Layers))
	}
	if manifest.Layers[0].Digest != "sha256:layer1" {
		t.Errorf("expected sha256:layer1, got %s", manifest.Layers[0].Digest)
	}
	if manifest.Layers[0].Size != 32654 {
		t.Errorf("expected size 32654, got %d", manifest.Layers[0].Size)
	}
}

func TestParseManifest_OCIV1(t *testing.T) {
	data := `{
		"schemaVersion": 2,
		"mediaType": "application/vnd.oci.image.manifest.v1+json",
		"config": {
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"size": 1234,
			"digest": "sha256:cfg123"
		},
		"layers": [
			{
				"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
				"size": 1000,
				"digest": "sha256:l1"
			},
			{
				"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
				"size": 2000,
				"digest": "sha256:l2"
			}
		]
	}`

	manifest, err := ParseManifest([]byte(data))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if len(manifest.Layers) != 2 {
		t.Errorf("expected 2 layers, got %d", len(manifest.Layers))
	}
}

func TestParseManifest_SchemaV1(t *testing.T) {
	data := `{"schemaVersion": 1, "name": "old-manifest"}`
	manifest, err := ParseManifest([]byte(data))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if manifest != nil {
		t.Error("expected nil for schema v1 manifest")
	}
}

func TestParseManifestList_Docker(t *testing.T) {
	data := `{
		"schemaVersion": 2,
		"mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
		"manifests": [
			{
				"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
				"size": 528,
				"digest": "sha256:amd64-manifest",
				"platform": {
					"architecture": "amd64",
					"os": "linux"
				}
			},
			{
				"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
				"size": 528,
				"digest": "sha256:arm64-manifest",
				"platform": {
					"architecture": "arm64",
					"os": "linux"
				}
			}
		]
	}`

	ml, err := ParseManifestList([]byte(data))
	if err != nil {
		t.Fatalf("ParseManifestList: %v", err)
	}
	if len(ml.Manifests) != 2 {
		t.Errorf("expected 2 manifests, got %d", len(ml.Manifests))
	}
	if ml.Manifests[0].Platform.Architecture != "amd64" {
		t.Errorf("expected amd64, got %s", ml.Manifests[0].Platform.Architecture)
	}
	if ml.Manifests[1].Platform.OS != "linux" {
		t.Errorf("expected linux, got %s", ml.Manifests[1].Platform.OS)
	}
}

func TestParseManifestList_Empty(t *testing.T) {
	data := `{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.index.v1+json", "manifests": []}`
	ml, err := ParseManifestList([]byte(data))
	if err != nil {
		t.Fatalf("ParseManifestList: %v", err)
	}
	if ml != nil {
		t.Error("expected nil for empty manifest list")
	}
}

func TestParseImageConfig_Basic(t *testing.T) {
	cfg := OCIImageConfig{
		Architecture: "amd64",
		OS:           "linux",
		Config: OCIConfigBlock{
			Env:        []string{"PATH=/usr/bin"},
			Cmd:        []string{"/bin/sh"},
			WorkingDir: "/",
		},
		RootFS: OCIRootFS{
			Type:    "layers",
			DiffIDs: []string{"sha256:l1"},
		},
	}

	data, _ := json.Marshal(cfg)
	parsed, err := ParseImageConfig(data)
	if err != nil {
		t.Fatalf("ParseImageConfig: %v", err)
	}
	if parsed.Architecture != "amd64" {
		t.Errorf("expected amd64, got %s", parsed.Architecture)
	}
	if len(parsed.Config.Cmd) != 1 || parsed.Config.Cmd[0] != "/bin/sh" {
		t.Errorf("expected [/bin/sh], got %v", parsed.Config.Cmd)
	}
}

func TestIsManifestList(t *testing.T) {
	tests := []struct {
		mediaType string
		want      bool
	}{
		{"application/vnd.oci.image.index.v1+json", true},
		{"application/vnd.docker.distribution.manifest.list.v2+json", true},
		{"application/vnd.docker.distribution.manifest.v2+json", false},
		{"application/vnd.oci.image.manifest.v1+json", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := IsManifestList(tt.mediaType); got != tt.want {
			t.Errorf("IsManifestList(%q) = %v, want %v", tt.mediaType, got, tt.want)
		}
	}
}
