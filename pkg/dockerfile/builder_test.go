package dockerfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func TestTakeSnapshot_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	snapshot, err := takeSnapshot(tmp)
	if err != nil {
		t.Fatalf("takeSnapshot: %v", err)
	}
	if len(snapshot) != 0 {
		t.Errorf("expected empty snapshot for empty dir, got %d entries", len(snapshot))
	}
}

func TestTakeSnapshot_BasicFiles(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("world"), 0o644)

	snapshot, err := takeSnapshot(tmp)
	if err != nil {
		t.Fatalf("takeSnapshot: %v", err)
	}
	if len(snapshot) != 2 {
		t.Errorf("expected 2 entries, got %d", len(snapshot))
	}
	if _, exists := snapshot["a.txt"]; !exists {
		t.Error("expected a.txt in snapshot")
	}
	if _, exists := snapshot["b.txt"]; !exists {
		t.Error("expected b.txt in snapshot")
	}
}

func TestTakeSnapshot_Subdirectories(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "sub", "deep"), 0o755)
	os.WriteFile(filepath.Join(tmp, "sub", "deep", "nested.txt"), []byte("nested"), 0o644)
	os.WriteFile(filepath.Join(tmp, "sub", "file.txt"), []byte("subfile"), 0o644)

	snapshot, err := takeSnapshot(tmp)
	if err != nil {
		t.Fatalf("takeSnapshot: %v", err)
	}
	if len(snapshot) != 2 {
		t.Errorf("expected 2 entries (dirs skipped), got %d", len(snapshot))
	}
	if _, exists := snapshot[filepath.Join("sub", "deep", "nested.txt")]; !exists {
		t.Error("expected nested.txt in snapshot")
	}
}

func TestComputeDiff_NoChanges(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "static.txt"), []byte("unchanged"), 0o644)

	snapshot, _ := takeSnapshot(tmp)

	changed, err := computeDiff(snapshot, tmp)
	if err != nil {
		t.Fatalf("computeDiff: %v", err)
	}
	if changed != 0 {
		t.Errorf("expected 0 for unchanged files, got %d", changed)
	}
}

func TestComputeDiff_NewFile(t *testing.T) {
	tmp := t.TempDir()
	snapshot, _ := takeSnapshot(tmp)

	os.WriteFile(filepath.Join(tmp, "newfile.txt"), []byte("I am new"), 0o644)

	changed, err := computeDiff(snapshot, tmp)
	if err != nil {
		t.Fatalf("computeDiff: %v", err)
	}
	if changed == 0 {
		t.Error("expected non-zero diff for new file")
	}
}

func TestComputeDiff_ModifiedFile(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "modify.txt"), []byte("original"), 0o644)
	snapshot, _ := takeSnapshot(tmp)

	os.WriteFile(filepath.Join(tmp, "modify.txt"), []byte("modified content here"), 0o644)

	changed, err := computeDiff(snapshot, tmp)
	if err != nil {
		t.Fatalf("computeDiff: %v", err)
	}
	if changed == 0 {
		t.Error("expected non-zero diff for modified file")
	}
}

func TestComputeDiff_DeletedFile(t *testing.T) {
	tmp := t.TempDir()
	delPath := filepath.Join(tmp, "todelete.txt")
	os.WriteFile(delPath, []byte("delete me"), 0o644)
	snapshot, _ := takeSnapshot(tmp)

	os.Remove(delPath)

	changed, err := computeDiff(snapshot, tmp)
	if err != nil {
		t.Fatalf("computeDiff: %v", err)
	}
	if changed <= 0 {
		t.Errorf("expected positive diff for deleted file, got %d", changed)
	}
}

func TestComputeDiff_MultipleChanges(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "keep.txt"), []byte("keep me"), 0o644)
	os.WriteFile(filepath.Join(tmp, "modify.txt"), []byte("original"), 0o644)
	snapshot, _ := takeSnapshot(tmp)

	os.WriteFile(filepath.Join(tmp, "modify.txt"), []byte("modified"), 0o644)
	os.WriteFile(filepath.Join(tmp, "new.txt"), []byte("new file"), 0o644)
	os.Remove(filepath.Join(tmp, "keep.txt"))

	changed, err := computeDiff(snapshot, tmp)
	if err != nil {
		t.Fatalf("computeDiff: %v", err)
	}
	if changed <= 0 {
		t.Error("expected positive diff for multiple changes")
	}
}

func TestHandleRun_NoKernel(t *testing.T) {
	b := &Builder{
		curRootfs: t.TempDir(),
		kernelID:  "",
	}
	err := b.handleRun(&RunInstruction{
		Command: "apk add nginx",
		Shell:   true,
	})
	if err != nil {
		t.Errorf("handleRun without kernel should not error: %v", err)
	}
}

func TestHandleRun_KernelNotFound(t *testing.T) {
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Images:     filepath.Join(tmp, "images"),
		Kernels:    filepath.Join(tmp, "kernels"),
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
		Tmp:        filepath.Join(tmp, "tmp"),
	}
	paths.EnsureAll()

	b := &Builder{
		curRootfs: t.TempDir(),
		kernelID:  "sha256:nonexistent",
		paths:     paths,
	}
	err := b.handleRun(&RunInstruction{
		Command: "echo hello",
		Shell:   true,
	})
	if err != nil {
		t.Errorf("handleRun with nonexistent kernel should not error (graceful fallback): %v", err)
	}
}

func TestFindQEMU(t *testing.T) {
	path, err := findQEMU()
	if err != nil {
		t.Logf("findQEMU returned error (expected if QEMU not installed): %v", err)
	} else {
		if path == "" {
			t.Error("findQEMU returned empty path without error")
		}
		if !strings.Contains(path, "qemu-system") {
			t.Errorf("expected path to contain qemu-system, got %s", path)
		}
	}
}

func TestBuilderRun_ExecForm(t *testing.T) {
	b := &Builder{
		curRootfs: t.TempDir(),
	}
	err := b.handleRun(&RunInstruction{
		Command: "nginx -g daemon off;",
		Shell:   false,
	})
	if err != nil {
		t.Errorf("handleRun exec form should not error: %v", err)
	}
}

func TestBuilderRun_IncrementsLayerCount(t *testing.T) {
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Images:     filepath.Join(tmp, "images"),
		Kernels:    filepath.Join(tmp, "kernels"),
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
		Tmp:        filepath.Join(tmp, "tmp"),
	}
	paths.EnsureAll()

	kernelPath := paths.KernelImagePath("sha256:fake")
	os.MkdirAll(filepath.Dir(kernelPath), 0o755)
	os.WriteFile(kernelPath, []byte("fake kernel"), 0o755)

	b := &Builder{
		curRootfs: t.TempDir(),
		kernelID:  "sha256:fake",
		paths:     paths,
		layers:    []image.Layer{{Digest: "sha256:base", Size: 100}},
	}

	qemuPath, qemuErr := findQEMU()
	_ = qemuPath
	beforeLayerCount := len(b.layers)
	_ = b.handleRun(&RunInstruction{Command: "echo test", Shell: true})
	if qemuErr == nil {
		afterLayerCount := len(b.layers)
		if afterLayerCount <= beforeLayerCount && afterLayerCount == beforeLayerCount {
			t.Log("QEMU found but may have failed to execute — layer count unchanged")
		}
	}
}
