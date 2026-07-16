package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cookiengineer/poqman/pkg/dockerfile"
	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func TestIntegration_DockerfileBuildAlpine(t *testing.T) {
	contextDir := t.TempDir()
	content := `FROM alpine:latest
ENV APP_NAME=poqman-integration-test
ENV APP_VERSION=1.0.0
WORKDIR /opt/app
EXPOSE 8080/tcp
LABEL com.poqman.test=true
CMD ["/bin/sh"]
`
	os.WriteFile(filepath.Join(contextDir, "Dockerfile"), []byte(content), 0o644)
	os.WriteFile(filepath.Join(contextDir, "hello.txt"), []byte("hello from poqman"), 0o644)

	tag := "localhost/poqman-test/alpine-build:latest"
	opts := dockerfile.BuildOptions{Tag: tag, ContextPath: contextDir}
	img, err := dockerfile.Build(opts)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if img.ID == "" {
		t.Fatal("expected non-empty image ID")
	}
	if len(img.RepoTags) == 0 || img.RepoTags[0] != tag {
		t.Errorf("expected tag %q, got %v", tag, img.RepoTags)
	}

	paths, _ := storage.ResolvePaths()
	paths.EnsureAll()
	imgStore := image.NewStore(paths)

	loaded, err := imgStore.Get(img.ID)
	if err != nil {
		t.Fatalf("get built image %s: %v", img.ID[:20], err)
	}

	if loaded.Config.Workdir != "/opt/app" {
		t.Errorf("expected Workdir /opt/app, got %q", loaded.Config.Workdir)
	}

	hasAppName := false
	for _, env := range loaded.Config.Env {
		if strings.Contains(env, "APP_NAME=poqman-integration-test") {
			hasAppName = true
		}
	}
	if !hasAppName {
		t.Error("expected APP_NAME env var")
	}

	if len(loaded.Config.Cmd) != 1 || loaded.Config.Cmd[0] != "/bin/sh" {
		t.Errorf("expected Cmd [/bin/sh], got %v", loaded.Config.Cmd)
	}

	if loaded.KernelRef != "" {
		t.Errorf("expected empty KernelRef (no KERNEL instruction), got %q", loaded.KernelRef)
	}

	t.Logf("Built %s (ID: %.20s, layers: %d)", tag, img.ID, len(loaded.Layers))

	cleanupImage(t, imgStore, tag, img.ID)
}

func cleanupImage(t *testing.T, imgStore *image.Store, tag string, imageID string) {
	t.Helper()
	idx, _ := imgStore.LoadIndex()
	idx.Remove(tag)
	imgStore.SaveIndex(idx)
	if err := imgStore.Remove(imageID); err != nil {
		t.Logf("cleanup: remove image %s: %v", imageID[:20], err)
	}
}

func TestIntegration_DockerfileBuildAlpineWithKernel(t *testing.T) {
	kernelRef := "debian:6.1.0-50-amd64:6.1.176-1"

	contextDir := t.TempDir()
	content := fmt.Sprintf(`FROM alpine:latest
KERNEL "%s"
ENV TEST_MODE=kernel-build
RUN echo "build-step-1" > /tmp/build-marker
COPY hello.txt /opt/hello.txt
CMD ["/bin/sh"]
`, kernelRef)
	os.WriteFile(filepath.Join(contextDir, "Dockerfile"), []byte(content), 0o644)
	os.WriteFile(filepath.Join(contextDir, "hello.txt"), []byte("hello with kernel"), 0o644)

	tag := "localhost/poqman-test/alpine-kernel:latest"
	opts := dockerfile.BuildOptions{Tag: tag, ContextPath: contextDir}
	img, err := dockerfile.Build(opts)
	if err != nil {
		t.Fatalf("Build with kernel failed: %v", err)
	}

	paths, _ := storage.ResolvePaths()
	imgStore := image.NewStore(paths)

	loaded, err := imgStore.Get(img.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if loaded.KernelRef != kernelRef {
		t.Errorf("expected KernelRef %q, got %q", kernelRef, loaded.KernelRef)
	}

	kernelBinPath := paths.ImageKernelPath(img.ID)
	if _, statErr := os.Stat(kernelBinPath); statErr == nil {
		t.Logf("Kernel binary present")
	}

	t.Logf("Built %s (ID: %.20s, kernel: %s)", tag, img.ID, loaded.KernelRef)

	cleanupImage(t, imgStore, tag, img.ID)
}

func TestIntegration_DockerfileBuildWithCopyAndIgnore(t *testing.T) {
	contextDir := t.TempDir()
	dockerfileContent := `FROM alpine:latest
WORKDIR /app
COPY . /app/
CMD ["/bin/sh"]
`
	dockerignoreContent := `*.log
*.tmp
.git/
node_modules/
`
	os.WriteFile(filepath.Join(contextDir, "Dockerfile"), []byte(dockerfileContent), 0o644)
	os.WriteFile(filepath.Join(contextDir, ".dockerignore"), []byte(dockerignoreContent), 0o644)
	os.WriteFile(filepath.Join(contextDir, "app.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(contextDir, "debug.log"), []byte("DEBUG: test log"), 0o644)
	os.WriteFile(filepath.Join(contextDir, "readme.md"), []byte("# Poqman Test"), 0o644)

	tag := "localhost/poqman-test/copy-ignore:latest"
	opts := dockerfile.BuildOptions{Tag: tag, ContextPath: contextDir}
	img, err := dockerfile.Build(opts)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	paths, _ := storage.ResolvePaths()
	paths.EnsureAll()
	imgStore := image.NewStore(paths)

	loaded, err := imgStore.Get(img.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if loaded.Config.Workdir != "/app" {
		t.Errorf("expected Workdir /app, got %q", loaded.Config.Workdir)
	}

	t.Logf("COPY+.dockerignore build: %s (ID: %.20s)", tag, img.ID)

	cleanupImage(t, imgStore, tag, img.ID)
}

func TestIntegration_DockerfileMultiInstructionBuild(t *testing.T) {
	contextDir := t.TempDir()
	content := `FROM alpine:latest
LABEL stage=builder
ARG BUILD_VERSION=1.0.0
ENV VERSION=${BUILD_VERSION}
ENV APP_ROOT=/srv/app
WORKDIR ${APP_ROOT}
EXPOSE 3000/tcp
EXPOSE 3001/tcp
VOLUME /data
VOLUME /logs
USER nobody
SHELL ["/bin/sh", "-c"]
ENTRYPOINT ["/entrypoint.sh"]
CMD ["--config", "/etc/app.conf"]
`
	os.WriteFile(filepath.Join(contextDir, "Dockerfile"), []byte(content), 0o644)

	tag := "localhost/poqman-test/multi-instr:latest"
	opts := dockerfile.BuildOptions{
		Tag:         tag,
		ContextPath: contextDir,
		BuildArgs:   map[string]string{"BUILD_VERSION": "2.5.0"},
	}
	img, err := dockerfile.Build(opts)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	paths, _ := storage.ResolvePaths()
	imgStore := image.NewStore(paths)
	loaded, _ := imgStore.Get(img.ID)

	if loaded.Config.User != "nobody" {
		t.Errorf("expected User nobody, got %q", loaded.Config.User)
	}
	if loaded.Config.Entrypoint == nil || len(loaded.Config.Entrypoint) == 0 {
		t.Error("expected non-nil Entrypoint")
	} else if loaded.Config.Entrypoint[0] != "/entrypoint.sh" {
		t.Errorf("expected /entrypoint.sh, got %v", loaded.Config.Entrypoint)
	}
	if loaded.Config.Volumes == nil {
		t.Error("expected Volumes")
	}
	if loaded.Config.Labels == nil || loaded.Config.Labels["stage"] != "builder" {
		t.Error("expected label stage=builder")
	}

	t.Logf("Multi-instruction build: %s (ID: %.20s)", tag, img.ID)

	cleanupImage(t, imgStore, tag, img.ID)
}

func TestIntegration_DockerfileBuildAndExport(t *testing.T) {
	contextDir := t.TempDir()
	content := `FROM alpine:latest
ENV EXPORT_TEST=true
CMD ["echo", "exported"]
`
	os.WriteFile(filepath.Join(contextDir, "Dockerfile"), []byte(content), 0o644)

	tag := "localhost/poqman-test/export:latest"
	opts := dockerfile.BuildOptions{Tag: tag, ContextPath: contextDir}
	img, err := dockerfile.Build(opts)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	paths, _ := storage.ResolvePaths()
	imgStore := image.NewStore(paths)

	exportPath := filepath.Join(t.TempDir(), "export.tar.gz")
	if err := image.SaveImage(img, paths, exportPath); err != nil {
		t.Fatalf("SaveImage: %v", err)
	}

	stat, _ := os.Stat(exportPath)
	if stat.Size() == 0 {
		t.Error("export archive is empty")
	}
	t.Logf("Exported %s (%d bytes)", tag, stat.Size())

	imported, err := image.LoadImage(paths, exportPath)
	if err != nil {
		t.Fatalf("LoadImage: %v", err)
	}
	if imported.ID != img.ID {
		t.Errorf("round-trip ID mismatch: %s != %s", imported.ID, img.ID)
	}

	t.Log("Dockerfile build → export → import round-trip OK")

	cleanupImage(t, imgStore, tag, img.ID)
}

