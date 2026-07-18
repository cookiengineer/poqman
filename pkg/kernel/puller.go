package kernel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/registry"
	"github.com/cookiengineer/poqman/pkg/storage"
)

type Puller struct {
	store    *Store
	paths    *storage.Paths
	images   *registry.Puller
	resolver *ResolverRegistry
}

func NewPuller(paths *storage.Paths) *Puller {
	return &Puller{
		store:    NewStore(paths),
		paths:    paths,
		images:   registry.NewPuller(paths),
		resolver: NewResolverRegistry(),
	}
}

func (p *Puller) Pull(rawRef string) (*Kernel, error) {
	req, err := ParseKernelRef(rawRef)
	if err != nil {
		return nil, err
	}

	if req.Arch == "" {
		req.Arch = hostArchKernel()
	}

	resolver, downloadURL, format, err := p.resolver.Resolve(req)
	if err != nil {
		return nil, fmt.Errorf("resolve kernel %q: %w", rawRef, err)
	}

	existing, err := p.store.Resolve(req)
	if err == nil && HasNinePModules(p.paths, existing.ID) {
		fmt.Fprintf(os.Stderr, "Kernel %s already cached (ID: %.20s)\n", rawRef, existing.ID)
		return existing, nil
	}

	if err == nil {
		fmt.Fprintf(os.Stderr, "Kernel %s cached but missing 9p modules, re-pulling...\n", rawRef)
	}

	fmt.Fprintf(os.Stderr, "Pulling kernel %s...\n", rawRef)

	var extractedDir string

	switch format {
	case "oci-image":
		extractedDir, err = p.pullOCIKernel(downloadURL, req)
		if err != nil {
			return nil, fmt.Errorf("pull OCI kernel: %w", err)
		}

	case "deb":
		extractedDir, err = p.downloadAndExtract(downloadURL, req, "deb")
		if err != nil {
			return nil, err
		}

	case "apk":
		extractedDir, err = p.downloadAndExtract(downloadURL, req, "apk")
		if err != nil {
			return nil, err
		}

	default:
		extractedDir, err = p.downloadAndExtract(downloadURL, req, format)
		if err != nil {
			return nil, err
		}
	}

	kernelBinPath, err := resolver.FindKernelInDir(extractedDir)
	if err != nil {
		return nil, fmt.Errorf("find kernel binary: %w", err)
	}

	kernelID := GenerateKernelID([]byte(rawRef + "_" + time.Now().String()))
	kernelStoreDir := p.paths.KernelPath(kernelID)
	kernelDestPath := p.paths.KernelImagePath(kernelID)

	if err := os.MkdirAll(kernelStoreDir, storage.DefaultPerms); err != nil {
		return nil, fmt.Errorf("create kernel store dir: %w", err)
	}

	if err := copyFile(kernelBinPath, kernelDestPath); err != nil {
		return nil, fmt.Errorf("copy kernel binary: %w", err)
	}

	copyKernelModules(extractedDir, p.paths.KernelModulesDir(kernelID))

	kernel := &Kernel{
		ID:         kernelID,
		Distro:     req.Distro,
		Version:    req.Version,
		Arch:       req.Arch,
		PackageURL: downloadURL,
		Created:    time.Now(),
	}

	if err := p.store.Save(kernel); err != nil {
		return nil, fmt.Errorf("save kernel: %w", err)
	}

	idx, _ := p.store.LoadIndex()
	idx.Add(req.String(), kernel.ID)
	p.store.SaveIndex(idx)

	fmt.Fprintf(os.Stderr, "  Kernel pulled: %s (ID: %.20s)\n", rawRef, kernel.ID)

	return kernel, nil
}

func (p *Puller) pullOCIKernel(imageRef string, req *ResolveRequest) (string, error) {
	ref, err := parseOCIImageRef(imageRef)
	if err != nil {
		return "", fmt.Errorf("parse OCI kernel ref: %w", err)
	}

	platform := registry.Platform{
		OS:           "linux",
		Architecture: req.Arch,
	}

	img, err := p.images.Pull(ref, platform)
	if err != nil {
		return "", fmt.Errorf("pull OCI kernel image: %w", err)
	}

	layerDir := p.paths.ImageLayersDir(img.ID)
	entries, err := os.ReadDir(layerDir)
	if err != nil {
		return "", fmt.Errorf("read image layers: %w", err)
	}

	if len(entries) > 0 {
		kernelPath, err := findKernelBinary(filepath.Join(layerDir, entries[0].Name()))
		if err == nil {
			return filepath.Dir(kernelPath), nil
		}
	}

	return layerDir, nil
}

func (p *Puller) downloadAndExtract(downloadURL string, req *ResolveRequest, format string) (string, error) {
	tmpDir := filepath.Join(p.paths.Tmp, "kernel-"+req.String())
	if err := os.MkdirAll(tmpDir, storage.DefaultPerms); err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	archivePath := filepath.Join(tmpDir, "kernel-package")
	fmt.Fprintf(os.Stderr, "  Downloading %s...\n", downloadURL)

	if err := DownloadArchive(downloadURL, archivePath); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("download: %w", err)
	}

	extractDir := filepath.Join(tmpDir, "extracted")
	fmt.Fprintf(os.Stderr, "  Extracting...\n")

	switch format {
	case "deb":
		if err := ExtractDeb(archivePath, extractDir); err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("extract deb: %w", err)
		}
		extractDir = filepath.Join(extractDir, "contents")
	case "apk":
		if err := ExtractApk(archivePath, extractDir); err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("extract apk: %w", err)
		}
	case "tar.xz":
		if err := extractTarXz(archivePath, extractDir); err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("extract tar.xz: %w", err)
		}
	case "tar.zst":
		if err := ExtractPkgTarZst(archivePath, extractDir); err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("extract tar.zst: %w", err)
		}
	default:
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("unsupported package format: %s", format)
	}

	return extractDir, nil
}

func copyKernelModules(extractedDir, modulesDest string) {
	filepath.Walk(extractedDir, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".ko") {
			return nil
		}

		relPath := strings.TrimPrefix(srcPath, extractedDir)
		relPath = strings.TrimPrefix(relPath, "/")

		is9p := strings.Contains(relPath, "fs/9p/") || strings.Contains(relPath, "net/9p/") ||
			strings.Contains(relPath, "drivers/virtio/") || strings.Contains(relPath, "fs/fscache/") ||
			strings.Contains(relPath, "fs/netfs/") || strings.Contains(relPath, "drivers/net/virtio_net") ||
			strings.Contains(relPath, "drivers/net/net_failover") || strings.Contains(relPath, "net/core/failover")
		if !is9p {
			return nil
		}

		baseName := filepath.Base(relPath)
		dest := filepath.Join(modulesDest, baseName)
		os.MkdirAll(filepath.Dir(dest), storage.DefaultPerms)
		copyFile(srcPath, dest)
		return nil
	})
}

func parseOCIImageRef(raw string) (image.ImageRef, error) {
	if strings.HasPrefix(raw, "oci://") {
		raw = raw[6:]
	}

	ref := image.ImageRef{
		Registry:   "docker.io",
		Repository: raw,
		Tag:        "latest",
	}

	if strings.Contains(raw, "/") {
		parts := strings.SplitN(raw, "/", 2)
		ref.Registry = parts[0]
		ref.Repository = parts[1]
	}

	if strings.Contains(ref.Repository, ":") {
		parts := strings.SplitN(ref.Repository, ":", 2)
		ref.Repository = parts[0]
		ref.Tag = parts[1]
	}

	return ref, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), storage.DefaultPerms); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}
	return os.WriteFile(dst, data, 0o755)
}
