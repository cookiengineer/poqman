package dockerfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateDiffLayer_Basic(t *testing.T) {
	tmp := t.TempDir()
	rootfs := filepath.Join(tmp, "rootfs")
	os.MkdirAll(rootfs, 0o755)
	os.WriteFile(filepath.Join(rootfs, "changed.txt"), []byte("changed content"), 0o644)
	os.WriteFile(filepath.Join(rootfs, "unchanged.txt"), []byte("same"), 0o644)

	snapshot, _ := takeSnapshot(rootfs)

	os.WriteFile(filepath.Join(rootfs, "changed.txt"), []byte("new content after build"), 0o644)
	os.WriteFile(filepath.Join(rootfs, "newfile.txt"), []byte("brand new"), 0o644)

	destPath := filepath.Join(tmp, "layer.tar.gz")
	size, err := createDiffLayer(snapshot, rootfs, destPath)
	if err != nil {
		t.Fatalf("createDiffLayer: %v", err)
	}
	if size <= 0 {
		t.Error("expected positive size for diff layer")
	}

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatal("layer.tar.gz should exist")
	}
	fileInfo, _ := os.Stat(destPath)
	if fileInfo.Size() == 0 {
		t.Error("layer.tar.gz should not be empty")
	}
}

func TestCreateDiffLayer_NoChanges(t *testing.T) {
	tmp := t.TempDir()
	rootfs := filepath.Join(tmp, "rootfs")
	os.MkdirAll(rootfs, 0o755)
	os.WriteFile(filepath.Join(rootfs, "static.txt"), []byte("unchanged"), 0o644)

	snapshot, _ := takeSnapshot(rootfs)

	destPath := filepath.Join(tmp, "layer.tar.gz")
	size, err := createDiffLayer(snapshot, rootfs, destPath)
	if err != nil {
		t.Fatalf("createDiffLayer: %v", err)
	}
	if size != 0 {
		t.Errorf("expected 0 size for no changes, got %d", size)
	}
}

func TestCreateDiffLayer_OnlyNewFiles(t *testing.T) {
	tmp := t.TempDir()
	rootfs := filepath.Join(tmp, "rootfs")
	os.MkdirAll(rootfs, 0o755)

	snapshot, _ := takeSnapshot(rootfs)

	os.WriteFile(filepath.Join(rootfs, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(rootfs, "b.txt"), []byte("bb"), 0o644)

	destPath := filepath.Join(tmp, "layer.tar.gz")
	size, err := createDiffLayer(snapshot, rootfs, destPath)
	if err != nil {
		t.Fatalf("createDiffLayer: %v", err)
	}
	if size != 3 {
		t.Errorf("expected size 3 (1+2 bytes), got %d", size)
	}
}

func TestCreateDiffLayer_SkipsBuildArtifacts(t *testing.T) {
	tmp := t.TempDir()
	rootfs := filepath.Join(tmp, "rootfs")
	os.MkdirAll(filepath.Join(rootfs, "tmp"), 0o755)
	os.WriteFile(filepath.Join(rootfs, "tmp", "poqman-exit-code"), []byte("0"), 0o644)
	os.WriteFile(filepath.Join(rootfs, "tmp", "poqman-build.sh"), []byte("script"), 0o644)
	os.WriteFile(filepath.Join(rootfs, "real.txt"), []byte("real content"), 0o644)

	snapshot, _ := takeSnapshot(rootfs)

	os.WriteFile(filepath.Join(rootfs, "tmp", "poqman-exit-code"), []byte("0\n"), 0o644)
	os.WriteFile(filepath.Join(rootfs, "real.txt"), []byte("modified"), 0o644)

	destPath := filepath.Join(tmp, "layer.tar.gz")
	size, err := createDiffLayer(snapshot, rootfs, destPath)
	if err != nil {
		t.Fatalf("createDiffLayer: %v", err)
	}
	if size <= 0 {
		t.Error("should include real.txt change")
	}
}

func TestExtractLayerFile_Basic(t *testing.T) {
	tmp := t.TempDir()
	rootfs := filepath.Join(tmp, "rootfs")
	os.MkdirAll(rootfs, 0o755)
	os.WriteFile(filepath.Join(rootfs, "file.txt"), []byte("hello layer"), 0o644)

	snapshot, _ := takeSnapshot(rootfs)
	os.WriteFile(filepath.Join(rootfs, "file.txt"), []byte("modified content"), 0o644)

	layerPath := filepath.Join(tmp, "layer.tar.gz")
	createDiffLayer(snapshot, rootfs, layerPath)

	extractDir := filepath.Join(tmp, "extracted")
	if err := extractLayerFile(layerPath, extractDir); err != nil {
		t.Fatalf("extractLayerFile: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(extractDir, "file.txt"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(data) != "modified content" {
		t.Errorf("expected 'modified content', got %q", string(data))
	}
}

func TestExtractLayerFile_Subdirectories(t *testing.T) {
	tmp := t.TempDir()
	rootfs := filepath.Join(tmp, "rootfs")
	os.MkdirAll(filepath.Join(rootfs, "usr", "bin"), 0o755)
	os.MkdirAll(filepath.Join(rootfs, "etc"), 0o755)

	snapshot, _ := takeSnapshot(rootfs)
	os.WriteFile(filepath.Join(rootfs, "usr", "bin", "app"), []byte("binary"), 0o755)
	os.WriteFile(filepath.Join(rootfs, "etc", "config.ini"), []byte("[main]\nkey=val\n"), 0o644)

	layerPath := filepath.Join(tmp, "layer.tar.gz")
	createDiffLayer(snapshot, rootfs, layerPath)

	extractDir := filepath.Join(tmp, "extracted")
	extractLayerFile(layerPath, extractDir)

	data, _ := os.ReadFile(filepath.Join(extractDir, "usr", "bin", "app"))
	if string(data) != "binary" {
		t.Error("extracted binary mismatch")
	}

	config, _ := os.ReadFile(filepath.Join(extractDir, "etc", "config.ini"))
	if string(config) != "[main]\nkey=val\n" {
		t.Error("extracted config mismatch")
	}
}
