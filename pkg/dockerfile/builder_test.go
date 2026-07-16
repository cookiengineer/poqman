package dockerfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func TestParseKernel_EmptyDockerfile(t *testing.T) {
	lines := []string{}
	df, err := Parse(lines)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(df.Instructions) != 0 {
		t.Errorf("expected 0 instructions, got %d", len(df.Instructions))
	}
}

func TestParseKernel_NoFromInstruction(t *testing.T) {
	lines := []string{
		"ENV PATH=/usr/bin",
		"CMD /bin/sh",
	}
	df, err := Parse(lines)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(df.Instructions) != 2 {
		t.Errorf("expected 2 instructions, got %d", len(df.Instructions))
	}
}

func TestBuilderHandleEnv(t *testing.T) {
	b := &Builder{
		imageConfig: image.ImageConfig{},
	}
	b.handleEnv(&EnvInstruction{Key: "PATH", Value: "/usr/bin"})
	b.handleEnv(&EnvInstruction{Key: "HOME", Value: "/root"})

	if len(b.imageConfig.Env) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(b.imageConfig.Env))
	}
	if b.imageConfig.Env[0] != "PATH=/usr/bin" {
		t.Errorf("expected PATH=/usr/bin, got %s", b.imageConfig.Env[0])
	}
	if b.imageConfig.Env[1] != "HOME=/root" {
		t.Errorf("expected HOME=/root, got %s", b.imageConfig.Env[1])
	}
}

func TestBuilderHandleCmd(t *testing.T) {
	b := &Builder{
		imageConfig: 	image.ImageConfig{},
	}
	b.handleCmd(&CmdInstruction{
		Command: []string{"nginx", "-g", "daemon off;"},
		Shell:   false,
	})

	if len(b.imageConfig.Cmd) != 3 {
		t.Errorf("expected 3 cmd args, got %d", len(b.imageConfig.Cmd))
	}
	if b.imageConfig.Cmd[0] != "nginx" {
		t.Errorf("expected nginx, got %s", b.imageConfig.Cmd[0])
	}
}

func TestBuilderHandleEntrypoint(t *testing.T) {
	b := &Builder{
		imageConfig: 	image.ImageConfig{},
	}
	b.handleEntrypoint(&EntrypointInstruction{
		Command: []string{"/docker-entrypoint.sh"},
		Shell:   false,
	})

	if b.imageConfig.Entrypoint == nil {
		t.Fatal("expected non-nil entrypoint")
	}
	if len(b.imageConfig.Entrypoint) != 1 {
		t.Errorf("expected 1 arg, got %d", len(b.imageConfig.Entrypoint))
	}
}

func TestBuilderHandleWorkdir(t *testing.T) {
	b := &Builder{
		imageConfig: 	image.ImageConfig{},
	}
	b.handleWorkdir(&WorkdirInstruction{Path: "/app/data"})

	if b.imageConfig.Workdir != "/app/data" {
		t.Errorf("expected /app/data, got %s", b.imageConfig.Workdir)
	}
}

func TestBuilderHandleExpose(t *testing.T) {
	b := &Builder{
		imageConfig: 	image.ImageConfig{},
	}
	b.handleExpose(&ExposeInstruction{Port: "80", Protocol: "tcp"})
	b.handleExpose(&ExposeInstruction{Port: "443", Protocol: "tcp"})
	b.handleExpose(&ExposeInstruction{Port: "53", Protocol: "udp"})

	ports := b.imageConfig.ExposedPorts
	if ports == nil {
		t.Fatal("expected non-nil exposed ports")
	}
	if len(ports) != 3 {
		t.Errorf("expected 3 ports, got %d", len(ports))
	}
	if _, exists := ports["80/tcp"]; !exists {
		t.Error("expected 80/tcp")
	}
	if _, exists := ports["53/udp"]; !exists {
		t.Error("expected 53/udp")
	}
}

func TestBuilderHandleVolume(t *testing.T) {
	b := &Builder{
		imageConfig: 	image.ImageConfig{},
	}
	b.handleVolume(&VolumeInstruction{Path: "/var/lib/data"})
	b.handleVolume(&VolumeInstruction{Path: "/var/log"})

	if b.imageConfig.Volumes == nil {
		t.Fatal("expected non-nil volumes")
	}
	if len(b.imageConfig.Volumes) != 2 {
		t.Errorf("expected 2 volumes, got %d", len(b.imageConfig.Volumes))
	}
}

func TestBuilderHandleUser(t *testing.T) {
	b := &Builder{
		imageConfig: 	image.ImageConfig{},
	}
	b.handleUser(&UserInstruction{User: "nginx"})

	if b.imageConfig.User != "nginx" {
		t.Errorf("expected nginx, got %s", b.imageConfig.User)
	}
}

func TestBuilderHandleLabel(t *testing.T) {
	b := &Builder{
		imageConfig: 	image.ImageConfig{},
	}
	b.handleLabel(&LabelInstruction{Key: "version", Value: "1.0"})
	b.handleLabel(&LabelInstruction{Key: "description", Value: "test image"})

	if len(b.imageConfig.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(b.imageConfig.Labels))
	}
	if b.imageConfig.Labels["version"] != "1.0" {
		t.Errorf("expected version=1.0, got %s", b.imageConfig.Labels["version"])
	}
}

func TestBuilderHandleShell(t *testing.T) {
	b := &Builder{
		imageConfig: 	image.ImageConfig{},
	}
	b.handleShell(&ShellInstruction{Shell: []string{"/bin/bash", "-c"}})

	if len(b.imageConfig.Shell) != 2 {
		t.Errorf("expected 2 shell parts, got %d", len(b.imageConfig.Shell))
	}
}

func TestBuilderHandleArg(t *testing.T) {
	b := &Builder{
		buildArgs: make(map[string]string),
	}
	b.handleArg(&ArgInstruction{Name: "VERSION", Default: "1.0"})
	b.handleArg(&ArgInstruction{Name: "RELEASE"})

	if b.buildArgs["VERSION"] != "1.0" {
		t.Errorf("expected VERSION=1.0, got %s", b.buildArgs["VERSION"])
	}
	if _, exists := b.buildArgs["RELEASE"]; exists {
		t.Error("expected RELEASE to be absent (no default)")
	}
}

func TestBuilderHandleArg_Override(t *testing.T) {
	b := &Builder{
		buildArgs: map[string]string{"VERSION": "2.0"},
	}
	b.handleArg(&ArgInstruction{Name: "VERSION", Default: "1.0"})

	if b.buildArgs["VERSION"] != "2.0" {
		t.Errorf("expected VERSION=2.0 (pre-set), got %s", b.buildArgs["VERSION"])
	}
}

func TestBuilderConfigAccumulation(t *testing.T) {
	b := &Builder{
		imageConfig: 	image.ImageConfig{},
		buildArgs:   make(map[string]string),
	}

	instrs := []Instruction{
		&EnvInstruction{Key: "PATH", Value: "/usr/bin"},
		&WorkdirInstruction{Path: "/app"},
		&UserInstruction{User: "nobody"},
		&ExposeInstruction{Port: "8080", Protocol: "tcp"},
		&VolumeInstruction{Path: "/data"},
		&LabelInstruction{Key: "app", Value: "poqman"},
		&CmdInstruction{Command: []string{"/bin/sh"}, Shell: false},
		&ShellInstruction{Shell: []string{"/bin/sh", "-c"}},
	}

	for _, instr := range instrs {
		switch i := instr.(type) {
		case *EnvInstruction:
			b.handleEnv(i)
		case *WorkdirInstruction:
			b.handleWorkdir(i)
		case *UserInstruction:
			b.handleUser(i)
		case *ExposeInstruction:
			b.handleExpose(i)
		case *VolumeInstruction:
			b.handleVolume(i)
		case *LabelInstruction:
			b.handleLabel(i)
		case *CmdInstruction:
			b.handleCmd(i)
		case *ShellInstruction:
			b.handleShell(i)
		}
	}

	if b.imageConfig.Workdir != "/app" {
		t.Errorf("expected /app, got %s", b.imageConfig.Workdir)
	}
	if b.imageConfig.User != "nobody" {
		t.Errorf("expected nobody, got %s", b.imageConfig.User)
	}
	if len(b.imageConfig.Cmd) != 1 {
		t.Errorf("expected 1 cmd, got %d", len(b.imageConfig.Cmd))
	}
	if len(b.imageConfig.ExposedPorts) != 1 {
		t.Errorf("expected 1 exposed port, got %d", len(b.imageConfig.ExposedPorts))
	}
	if len(b.imageConfig.Volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(b.imageConfig.Volumes))
	}
}

func TestCopyFileContents(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.txt")
	dst := filepath.Join(tmp, "subdir", "dst.txt")

	os.WriteFile(src, []byte("test content"), 0o644)

	if err := copyFileContents(src, dst); err != nil {
		t.Fatalf("copyFileContents: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != "test content" {
		t.Errorf("expected 'test content', got %q", string(data))
	}
}

func TestCopyPath_File(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.txt")
	dst := filepath.Join(tmp, "dest")

	os.WriteFile(src, []byte("hello"), 0o644)
	os.MkdirAll(dst, 0o755)

	if err := copyPath(src, dst); err != nil {
		t.Fatalf("copyPath file: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dst, "src.txt"))
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}

func TestCopyPath_Dir(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "srcdir")
	srcSub := filepath.Join(src, "subdir")
	dst := filepath.Join(tmp, "dstdir")

	os.MkdirAll(srcSub, 0o755)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(srcSub, "b.txt"), []byte("b"), 0o644)

	if err := copyPath(src, dst); err != nil {
		t.Fatalf("copyPath dir: %v", err)
	}

	a, _ := os.ReadFile(filepath.Join(dst, "a.txt"))
	if string(a) != "a" {
		t.Errorf("expected 'a', got %q", string(a))
	}

	b, _ := os.ReadFile(filepath.Join(dst, "subdir", "b.txt"))
	if string(b) != "b" {
		t.Errorf("expected 'b', got %q", string(b))
	}
}

func TestCopyDirContents(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")

	os.MkdirAll(filepath.Join(src, "sub"), 0o755)
	os.WriteFile(filepath.Join(src, "root.txt"), []byte("root"), 0o644)
	os.WriteFile(filepath.Join(src, "sub", "sub.txt"), []byte("sub"), 0o644)

	if err := copyDirContents(src, dst); err != nil {
		t.Fatalf("copyDirContents: %v", err)
	}

	root, _ := os.ReadFile(filepath.Join(dst, "root.txt"))
	if string(root) != "root" {
		t.Errorf("expected 'root', got %q", string(root))
	}

	sub, _ := os.ReadFile(filepath.Join(dst, "sub", "sub.txt"))
	if string(sub) != "sub" {
		t.Errorf("expected 'sub', got %q", string(sub))
	}
}

func TestDirSize(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("12345"), 0o644)
	os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("1234567890"), 0o644)

	size := dirSize(tmp)
	if size != 15 {
		t.Errorf("expected size 15, got %d", size)
	}

	emptyDir := filepath.Join(tmp, "empty")
	os.MkdirAll(emptyDir, 0o755)
	size = dirSize(emptyDir)
	if size != 0 {
		t.Errorf("expected size 0 for empty dir, got %d", size)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world this is long", 10, "hello w..."},
		{"abcdef", 5, "ab..."},
		{"ab", 5, "ab"},
		{"exactly10", 10, "exactly10"},
	}

	for _, tt := range tests {
		got := truncate(tt.s, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
		}
	}
}

func TestBuilderRun_Recording(t *testing.T) {
	b := &Builder{
		curRootfs: t.TempDir(),
	}
	err := b.handleRun(&RunInstruction{
		Command: "apk add nginx",
		Shell:   true,
	})
	if err != nil {
		t.Errorf("handleRun should not error in MVP: %v", err)
	}
}

func TestBuilderHandleCopy_Integration(t *testing.T) {
	tmp := t.TempDir()
	contextDir := filepath.Join(tmp, "context")
	rootfsDir := filepath.Join(tmp, "rootfs")
	os.MkdirAll(rootfsDir, storage.DefaultPerms)
	os.MkdirAll(contextDir, storage.DefaultPerms)

	os.WriteFile(filepath.Join(contextDir, "hello.txt"), []byte("hello poqman"), 0o644)

	b := &Builder{
		contextPath: contextDir,
		curRootfs:   rootfsDir,
	}

	err := b.handleCopy(&CopyInstruction{
		Sources:     []string{"hello.txt"},
		Destination: "/app/",
	})
	if err != nil {
		t.Fatalf("handleCopy: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rootfsDir, "app", "hello.txt"))
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(data) != "hello poqman" {
		t.Errorf("expected 'hello poqman', got %q", string(data))
	}
}

func TestBuilderHandleAdd_Integration(t *testing.T) {
	tmp := t.TempDir()
	contextDir := filepath.Join(tmp, "context")
	rootfsDir := filepath.Join(tmp, "rootfs")
	os.MkdirAll(rootfsDir, storage.DefaultPerms)
	os.MkdirAll(contextDir, storage.DefaultPerms)

	os.WriteFile(filepath.Join(contextDir, "config.json"), []byte(`{"debug":true}`), 0o644)

	b := &Builder{
		contextPath: contextDir,
		curRootfs:   rootfsDir,
	}

	err := b.handleAdd(&AddInstruction{
		Sources:     []string{"config.json"},
		Destination: "/etc/",
	})
	if err != nil {
		t.Fatalf("handleAdd: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(rootfsDir, "etc", "config.json"))
	if string(data) != `{"debug":true}` {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestBuilderCommit_NoKernel(t *testing.T) {
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

	rootfsPath := filepath.Join(tmp, "rootfs")
	os.MkdirAll(rootfsPath, storage.DefaultPerms)
	os.WriteFile(filepath.Join(rootfsPath, "hello"), []byte("world"), 0o644)

	b := &Builder{
		tag:        "localhost/test:commit",
		workingDir: tmp,
		curRootfs:  rootfsPath,
		imageConfig: image.ImageConfig{
			Cmd: []string{"/bin/sh"},
		},
		layers: []image.Layer{
			{Digest: "sha256:base", Size: 100, MediaType: "test"},
		},
		paths: paths,
		arch:  "amd64",
	}

	img, err := b.commit()
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if img.ID == "" {
		t.Error("expected non-empty image ID")
	}
	if img.Arch != "amd64" {
		t.Errorf("expected amd64, got %s", img.Arch)
	}
	if len(img.RepoTags) == 0 || img.RepoTags[0] != "localhost/test:commit" {
		t.Errorf("unexpected repoTags: %v", img.RepoTags)
	}
	if img.KernelRef != "" {
		t.Errorf("expected empty kernelRef, got %s", img.KernelRef)
	}
}
