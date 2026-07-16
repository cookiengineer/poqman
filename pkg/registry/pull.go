package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/storage"
)

type Puller struct {
	client     *Client
	imageStore *image.Store
	paths      *storage.Paths
}

func NewPuller(paths *storage.Paths) *Puller {
	return &Puller{
		client:     NewClient(),
		imageStore: image.NewStore(paths),
		paths:      paths,
	}
}

func (p *Puller) Pull(ref image.ImageRef, platform Platform) (*image.Image, error) {
	existingImg, err := p.imageStore.Resolve(ref)
	if err == nil {
		fmt.Fprintf(os.Stderr, "Image %s already exists locally (ID: %.20s)\n", ref.FullName(), existingImg.ID)
		return existingImg, nil
	}

	fmt.Fprintf(os.Stderr, "Pulling %s...\n", ref.FullName())

	manifest, err := p.resolveManifest(ref, platform)
	if err != nil {
		return nil, fmt.Errorf("resolve manifest for %s: %w", ref.FullName(), err)
	}

	fmt.Fprintf(os.Stderr, "  Manifest: %d layers\n", len(manifest.Layers))

	cfgData, err := p.fetchBlob(ref.Registry, ref.Repository, manifest.Config.Digest)
	if err != nil {
		return nil, fmt.Errorf("fetch image config: %w", err)
	}

	ociConfig, err := ParseImageConfig(cfgData)
	if err != nil {
		return nil, fmt.Errorf("parse image config: %w", err)
	}

	imageID := "sha256:" + sha256Hash(cfgData)

	existingImg, err = p.imageStore.Get(imageID)
	if err == nil {
		idx, _ := p.imageStore.LoadIndex()
		idx.Add(ref.FullName(), imageID)
		p.imageStore.SaveIndex(idx)
		fmt.Fprintf(os.Stderr, "  Image %s already cached (ID: %.20s)\n", ref.FullName(), imageID)
		return existingImg, nil
	}

	var layers []image.Layer
	var totalSize int64

	for i, layer := range manifest.Layers {
		fmt.Fprintf(os.Stderr, "  Layer %d/%d: %s [%.2f MB]\n",
			i+1,
			len(manifest.Layers),
			shortDigest(layer.Digest),
			float64(layer.Size)/(1024*1024),
		)

		destDir := p.paths.ImageLayerPath(imageID, layer.Digest)
		if err := os.MkdirAll(destDir, storage.DefaultPerms); err != nil {
			return nil, fmt.Errorf("create layer dir: %w", err)
		}

		blobReader, _, err := p.client.GetBlob(ref.Registry, ref.Repository, layer.Digest)
		if err != nil {
			return nil, fmt.Errorf("download layer %s: %w", shortDigest(layer.Digest), err)
		}

		if err := storage.ExtractLayer(blobReader, layer.Digest, destDir); err != nil {
			return nil, fmt.Errorf("extract layer %s: %w", shortDigest(layer.Digest), err)
		}

		layers = append(layers, image.Layer{
			Digest:    layer.Digest,
			Size:      layer.Size,
			MediaType: layer.MediaType,
		})
		totalSize += layer.Size
	}

	img := &image.Image{
		ID:       imageID,
		RepoTags: []string{ref.FullName()},
		Arch:     ociConfig.Architecture,
		Config:   convertConfig(ociConfig),
		Layers:   layers,
		Created:  time.Now(),
		Size:     totalSize,
	}

	if err := p.imageStore.Save(img); err != nil {
		return nil, fmt.Errorf("save image: %w", err)
	}

	idx, err := p.imageStore.LoadIndex()
	if err != nil {
		return nil, fmt.Errorf("load image index: %w", err)
	}
	idx.Add(ref.FullName(), imageID)
	if err := p.imageStore.SaveIndex(idx); err != nil {
		return nil, fmt.Errorf("save image index: %w", err)
	}

	fmt.Fprintf(os.Stderr, "  Pulled: %s (ID: %.20s, %d layers)\n", ref.FullName(), imageID, len(layers))

	return img, nil
}

func (p *Puller) resolveManifest(ref image.ImageRef, platform Platform) (*Manifest, error) {
	rawData, mediaType, err := p.client.GetManifest(ref.Registry, ref.Repository, ref.Tag)
	if err != nil {
		return nil, err
	}

	if IsManifestList(mediaType) {
		manifestList, err := ParseManifestList(rawData)
		if err != nil {
			return nil, fmt.Errorf("parse manifest list: %w", err)
		}

		entry, found := MatchManifest(manifestList.Manifests, platform)
		if !found {
			return nil, fmt.Errorf("no manifest found for platform %s in %s", platform, ref.FullName())
		}

		fmt.Fprintf(os.Stderr, "  Found %s/%s manifest in image index\n",
			entry.Platform.OS, entry.Platform.Architecture)

		rawData, _, err = p.client.GetManifest(ref.Registry, ref.Repository, entry.Digest)
		if err != nil {
			return nil, fmt.Errorf("fetch platform manifest: %w", err)
		}
	}

	manifest, err := ParseManifest(rawData)
	if err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if manifest == nil {
		return nil, fmt.Errorf("unsupported manifest schema")
	}

	return manifest, nil
}

func (p *Puller) fetchBlob(registry, repo, digest string) ([]byte, error) {
	reader, _, err := p.client.GetBlob(registry, repo, digest)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read blob: %w", err)
	}

	actual := sha256Hash(data)
	expected := strings.TrimPrefix(digest, "sha256:")
	if actual != expected {
		return nil, fmt.Errorf("digest mismatch for %s: expected %s, got %s",
			shortDigest(digest), expected[:12], actual[:12])
	}

	return data, nil
}

func convertConfig(cfg *OCIImageConfig) image.ImageConfig {
	exposedPorts := make(map[string]struct{})
	if cfg.Config.ExposedPorts != nil {
		exposedPorts = cfg.Config.ExposedPorts
	}
	volumes := make(map[string]struct{})
	if cfg.Config.Volumes != nil {
		volumes = cfg.Config.Volumes
	}
	labels := make(map[string]string)
	if cfg.Config.Labels != nil {
		labels = cfg.Config.Labels
	}

	return image.ImageConfig{
		User:         cfg.Config.User,
		Env:          cfg.Config.Env,
		Cmd:          cfg.Config.Cmd,
		Entrypoint:   cfg.Config.Entrypoint,
		Workdir:      cfg.Config.WorkingDir,
		ExposedPorts: exposedPorts,
		Volumes:      volumes,
		Labels:       labels,
		StopSignal:   cfg.Config.StopSignal,
		Shell:        cfg.Config.Shell,
	}
}

func sha256Hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func shortDigest(digest string) string {
	if len(digest) <= 19 {
		return digest
	}
	return digest[:19]
}


