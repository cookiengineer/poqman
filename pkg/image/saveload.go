package image

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cookiengineer/poqman/pkg/storage"
)

func SaveImage(img *Image, paths *storage.Paths, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	manifestData, _ := json.Marshal(img)
	manifestHeader := &tar.Header{
		Name:    "manifest.json",
		Size:    int64(len(manifestData)),
		Mode:    0o644,
		ModTime: time.Now(),
	}
	tw.WriteHeader(manifestHeader)
	tw.Write(manifestData)

	for _, layer := range img.Layers {
		layerDir := paths.ImageLayerPath(img.ID, layer.Digest)
		tarPath := filepath.Join(layerDir, "layer.tar.gz")
		if _, err := os.Stat(tarPath); err == nil {
			addFileToTar(tw, tarPath, layer.Digest+"/layer.tar.gz")
		} else {
			err := filepath.Walk(layerDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(layerDir, path)
				return addFileToTar(tw, path, layer.Digest+"/"+rel)
			})
			if err != nil {
				return fmt.Errorf("archive layer %s: %w", layer.Digest, err)
			}
		}
	}

	kernelPath := paths.ImageKernelPath(img.ID)
	if _, err := os.Stat(kernelPath); err == nil {
		addFileToTar(tw, kernelPath, "kernel/bzImage")
	}

	return nil
}

func LoadImage(paths *storage.Paths, inputPath string) (*Image, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("open input file: %w", err)
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	var img *Image
	tmpDir := filepath.Join(paths.Tmp, "load-"+time.Now().Format("20060102150405"))
	os.MkdirAll(tmpDir, storage.DefaultPerms)
	defer os.RemoveAll(tmpDir)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar entry: %w", err)
		}

		target := filepath.Join(tmpDir, filepath.Clean(header.Name))

		switch {
		case header.Name == "manifest.json":
			var manifest Image
			if err := json.NewDecoder(tr).Decode(&manifest); err != nil {
				return nil, fmt.Errorf("decode manifest: %w", err)
			}
			img = &manifest

		case header.Typeflag == tar.TypeDir:
			os.MkdirAll(target, 0o755)

		default:
			os.MkdirAll(filepath.Dir(target), 0o755)
			out, err := os.Create(target)
			if err != nil {
				return nil, fmt.Errorf("create %s: %w", target, err)
			}
			io.Copy(out, tr)
			out.Close()
		}
	}

	if img == nil {
		return nil, fmt.Errorf("no manifest.json found in archive")
	}

	return img, nil
}

func addFileToTar(tw *tar.Writer, srcPath, tarName string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = strings.ReplaceAll(tarName, string(filepath.Separator), "/")

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(tw, file)
	return err
}
