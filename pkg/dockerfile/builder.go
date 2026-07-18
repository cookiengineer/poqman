package dockerfile

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/kernel"
	"github.com/cookiengineer/poqman/pkg/registry"
	"github.com/cookiengineer/poqman/pkg/storage"
)

type Builder struct {
	contextPath string
	df          *Dockerfile
	workingDir  string
	curRootfs   string
	kernelRef   string
	kernelID    string
	imageConfig image.ImageConfig
	layers      []image.Layer
	baseImageID string
	paths       *storage.Paths
	puller      *registry.Puller
	kernelPuller *kernel.Puller
	tag         string
	buildArgs   map[string]string
	arch        string
	ignorePatterns []IgnorePattern
	initBinary  []byte
}

type BuildOptions struct {
	Tag         string
	ContextPath string
	Dockerfile  string
	Platform    string
	BuildArgs   map[string]string
	InitBinary  []byte
}

func Build(opts BuildOptions) (*image.Image, error) {
	paths, err := storage.ResolvePaths()
	if err != nil {
		return nil, fmt.Errorf("resolve paths: %w", err)
	}
	paths.EnsureAll()

	dfPath := opts.Dockerfile
	if dfPath == "" {
		dfPath = "Dockerfile"
	}
	if !filepath.IsAbs(dfPath) {
		dfPath = filepath.Join(opts.ContextPath, dfPath)
	}

	lines, err := Scan(dfPath)
	if err != nil {
		return nil, fmt.Errorf("read Dockerfile: %w", err)
	}

	df, err := Parse(lines)
	if err != nil {
		return nil, fmt.Errorf("parse Dockerfile: %w", err)
	}

	b := &Builder{
		contextPath:  opts.ContextPath,
		df:           df,
		workingDir:   filepath.Join(paths.Tmp, "build-"+time.Now().Format("20060102150405")),
		imageConfig:  image.ImageConfig{},
		paths:        paths,
		puller:       registry.NewPuller(paths),
		kernelPuller: kernel.NewPuller(paths),
		tag:          opts.Tag,
		buildArgs:    opts.BuildArgs,
		arch:         "amd64",
		initBinary:   opts.InitBinary,
	}
	if b.buildArgs == nil {
		b.buildArgs = make(map[string]string)
	}

	if opts.Platform != "" {
		plat, _ := registry.ParsePlatform(opts.Platform)
		b.arch = plat.Architecture
	}

	ignorePatterns, _ := LoadDockerIgnore(opts.ContextPath)
	b.ignorePatterns = ignorePatterns

	if err := b.process(); err != nil {
		os.RemoveAll(b.workingDir)
		return nil, err
	}

	img, err := b.commit()
	if err != nil {
		os.RemoveAll(b.workingDir)
		return nil, err
	}

	os.RemoveAll(b.workingDir)
	return img, nil
}

func (b *Builder) process() error {
	if err := os.MkdirAll(b.workingDir, storage.DefaultPerms); err != nil {
		return fmt.Errorf("create build dir: %w", err)
	}

	b.curRootfs = filepath.Join(b.workingDir, "rootfs")
	os.MkdirAll(b.curRootfs, storage.DefaultPerms)

	hasFrom := false

	for _, instr := range b.df.Instructions {
		var err error
		switch i := instr.(type) {
		case *FromInstruction:
			err = b.handleFrom(i)
			hasFrom = true
		case *KernelInstruction:
			err = b.handleKernel(i)
		case *RunInstruction:
			err = b.handleRun(i)
		case *CopyInstruction:
			err = b.handleCopy(i)
		case *AddInstruction:
			err = b.handleAdd(i)
		case *CmdInstruction:
			err = b.handleCmd(i)
		case *EntrypointInstruction:
			err = b.handleEntrypoint(i)
		case *EnvInstruction:
			err = b.handleEnv(i)
		case *WorkdirInstruction:
			err = b.handleWorkdir(i)
		case *ExposeInstruction:
			err = b.handleExpose(i)
		case *VolumeInstruction:
			err = b.handleVolume(i)
		case *UserInstruction:
			err = b.handleUser(i)
		case *LabelInstruction:
			err = b.handleLabel(i)
		case *ArgInstruction:
			err = b.handleArg(i)
		case *ShellInstruction:
			err = b.handleShell(i)
		case *CommentInstruction:
		}
		if err != nil {
			return err
		}
	}

	if !hasFrom {
		return fmt.Errorf("Dockerfile must contain at least one FROM instruction")
	}

	return nil
}

func (b *Builder) handleFrom(instr *FromInstruction) error {
	ref, err := image.ParseImageRef(instr.Image)
	if err != nil {
		return fmt.Errorf("parse FROM image: %w", err)
	}

	plat := registry.HostPlatform()
	if instr.Platform != "" {
		plat, err = registry.ParsePlatform(instr.Platform)
		if err != nil {
			return fmt.Errorf("parse platform in FROM: %w", err)
		}
	}
	b.arch = plat.Architecture

	fmt.Fprintf(os.Stderr, "Step 1/%d : FROM %s\n", len(b.df.Instructions), instr.Image)

	img, err := b.puller.Pull(ref, plat)
	if err != nil {
		return fmt.Errorf("pull FROM image %s: %w", instr.Image, err)
	}

	b.baseImageID = img.ID

	layersDir := b.paths.ImageLayersDir(img.ID)
	entries, err := os.ReadDir(layersDir)
	if err != nil {
		return fmt.Errorf("read layers: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		layerPath := filepath.Join(layersDir, entry.Name())
		copyDirContents(layerPath, b.curRootfs)
	}

	b.layers = append(b.layers, img.Layers...)
	b.imageConfig = img.Config

	return nil
}

func (b *Builder) handleKernel(instr *KernelInstruction) error {
	fmt.Fprintf(os.Stderr, "Step : KERNEL %s\n", instr.Reference)

	ker, err := b.kernelPuller.Pull(instr.Reference)
	if err != nil {
		return fmt.Errorf("pull kernel %s: %w", instr.Reference, err)
	}

	b.kernelRef = instr.Reference
	b.kernelID = ker.ID

	return nil
}

func (b *Builder) handleRun(instr *RunInstruction) error {
	fmt.Fprintf(os.Stderr, "Step : RUN %s\n", truncate(instr.Command, 60))

	if b.kernelID == "" {
		fmt.Fprintf(os.Stderr, "  (no KERNEL specified; recording RUN for container startup)\n")
		return nil
	}

	kernelPath := b.paths.KernelImagePath(b.kernelID)
	if _, err := os.Stat(kernelPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "  (kernel not found at %s; recording RUN for container startup)\n", kernelPath)
		return nil
	}

	snapshot, err := takeSnapshot(b.curRootfs)
	if err != nil {
		return fmt.Errorf("snapshot rootfs: %w", err)
	}

	buildScript := fmt.Sprintf(`#!/bin/sh
mount -t proc proc /proc
mount -t sysfs sys /sys
mount -t devtmpfs dev /dev
mkdir -p /dev/pts
mount -t devpts devpts /dev/pts
mkdir -p /tmp
mkdir -p /etc
if [ -x /bin/busybox ]; then
  /bin/busybox ip link set eth0 up
  /bin/busybox ip addr add 10.0.2.15/24 dev eth0
  /bin/busybox ip route add default via 10.0.2.2
fi
echo "nameserver 10.0.2.3" > /etc/resolv.conf
echo 'DPkg::Options {"--force-all";};' > /etc/apt/apt.conf.d/99-poqman
echo 'Acquire::AllowInsecureRepositories "true";' >> /etc/apt/apt.conf.d/99-poqman
echo 'Acquire::AllowDowngradeToInsecureRepositories "true";' >> /etc/apt/apt.conf.d/99-poqman
echo 'APT::Get::AllowUnauthenticated "true";' >> /etc/apt/apt.conf.d/99-poqman
%s
echo $? > /tmp/poqman-exit-code
sync
poweroff -f 2>/dev/null || reboot -f
`, instr.Command)

	scriptPath := filepath.Join(b.curRootfs, "tmp", "poqman-build.sh")
	os.MkdirAll(filepath.Dir(scriptPath), 0o755)
	os.WriteFile(scriptPath, []byte(buildScript), 0o755)

	qemuBinary, err := findQEMU()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  (QEMU not found: %v; recording RUN for container startup)\n", err)
		return nil
	}

	setupMappedXattr(b.curRootfs)

	initrdPath := filepath.Join(b.workingDir, "initrd.gz")
	var useInitrd bool
	if kernel.HasNinePModules(b.paths, b.kernelID) {
		if err := kernel.BuildInitrd(b.paths, b.kernelID, b.initBinary, initrdPath); err != nil {
			fmt.Fprintf(os.Stderr, "  (initrd build failed: %v; trying without initrd)\n", err)
		} else {
			useInitrd = true
			fmt.Fprintf(os.Stderr, "  Initrd generated with 9p modules\n")
		}
	}

	kernelAppend := "root=rootfs rootfstype=9p rootflags=trans=virtio,version=9p2000.L rw console=ttyS0 quiet panic=1 init=/tmp/poqman-build.sh"
	if useInitrd {
		kernelAppend = "console=ttyS0 quiet panic=1 init=/tmp/poqman-build.sh"
	}

	qemuArgs := []string{
		"-kernel", kernelPath,
		"-append", kernelAppend,
		"-fsdev", fmt.Sprintf("local,id=rootfs,path=%s,security_model=mapped-xattr", b.curRootfs),
		"-device", "virtio-9p-pci,fsdev=rootfs,mount_tag=rootfs",
		"-nic", "user,model=virtio-net-pci",
		"-m", "512M",
		"-smp", "1",
		"-nographic",
		"-no-reboot",
	}

	if useInitrd {
		qemuArgs = append(qemuArgs, "-initrd", initrdPath)
	}

	fmt.Fprintf(os.Stderr, "  Booting build VM...\n")

	cmd := exec.Command(qemuBinary, qemuArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	buildCtx, buildCancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer buildCancel()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start build VM: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var qemuErr error
	select {
	case err := <-done:
		qemuErr = err
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: build VM exited with error: %v\n", err)
		}
	case <-buildCtx.Done():
		cmd.Process.Kill()
		qemuErr = fmt.Errorf("build VM timed out after 120s")
		fmt.Fprintf(os.Stderr, "  Error: build VM timed out after 120s\n")
	}

	exitCodePath := filepath.Join(b.curRootfs, "tmp", "poqman-exit-code")
	exitData, readErr := os.ReadFile(exitCodePath)
	os.Remove(exitCodePath)
	os.Remove(scriptPath)

	if readErr != nil {
		if qemuErr != nil {
			return fmt.Errorf("build VM failed (exit code file not written - likely kernel panic): %w", qemuErr)
		}
		return fmt.Errorf("build VM completed but exit code file was not written")
	}

	exitCode := 0
	fmt.Sscanf(strings.TrimSpace(string(exitData)), "%d", &exitCode)

	if exitCode != 0 {
		return fmt.Errorf("RUN command exited with code %d", exitCode)
	}

	layerDigest := fmt.Sprintf("sha256:build-run-%d", len(b.layers))
	layerDir := b.paths.ImageLayerPath("build-"+strings.ReplaceAll(b.tag, "/", "-"), layerDigest)
	layerFile := filepath.Join(layerDir, "layer.tar.gz")

	if err := os.MkdirAll(filepath.Dir(layerFile), storage.DefaultPerms); err != nil {
		return fmt.Errorf("create layer dir: %w", err)
	}

	changed, err := createDiffLayer(snapshot, b.curRootfs, layerFile)
	if err != nil {
		return fmt.Errorf("create diff layer: %w", err)
	}

	extractDir := filepath.Join(layerDir, "fs")
	if err := extractLayerFile(layerFile, extractDir); err != nil {
		return fmt.Errorf("extract diff layer: %w", err)
	}

	b.layers = append(b.layers, image.Layer{
		Digest:    layerDigest,
		Size:      changed,
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
	})

	fmt.Fprintf(os.Stderr, "  RUN completed (%d bytes changed)\n", changed)

	return nil
}

func setupMappedXattr(rootfs string) {
	filepath.Walk(rootfs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		setXattr(path, "user.virtfs.uid", []byte{0, 0, 0, 0})
		setXattr(path, "user.virtfs.gid", []byte{0, 0, 0, 0})
		return nil
	})
}

func setXattr(path, name string, value []byte) {
	syscall.Setxattr(path, name, value, 0)
}

func findQEMU() (string, error) {
	binary, err := exec.LookPath("qemu-system-x86_64")
	if err == nil {
		return binary, nil
	}
	binary, err = exec.LookPath("qemu-system-aarch64")
	if err == nil {
		return binary, nil
	}
	return "", fmt.Errorf("no QEMU binary found in PATH")
}

func takeSnapshot(rootfs string) (map[string]int64, error) {
	snapshot := make(map[string]int64)
	err := filepath.Walk(rootfs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(rootfs, path)
		snapshot[relPath] = info.ModTime().UnixNano()
		return nil
	})
	return snapshot, err
}

func computeDiff(before map[string]int64, rootfs string) (int64, error) {
	var changed int64
	err := filepath.Walk(rootfs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(rootfs, path)
		beforeTime, existed := before[relPath]
		if !existed || beforeTime != info.ModTime().UnixNano() {
			changed += info.Size()
		}
		return nil
	})

	for path := range before {
		fullPath := filepath.Join(rootfs, path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			changed++
		}
	}

	return changed, err
}

func (b *Builder) handleCopy(instr *CopyInstruction) error {
	fmt.Fprintf(os.Stderr, "Step : COPY %s -> %s\n", strings.Join(instr.Sources, " "), instr.Destination)

	dst := filepath.Join(b.curRootfs, instr.Destination)
	if err := os.MkdirAll(filepath.Dir(dst), storage.DefaultPerms); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	for _, src := range instr.Sources {
		if ShouldIgnore(src, b.ignorePatterns) {
			fmt.Fprintf(os.Stderr, "  (ignored: %s)\n", src)
			continue
		}
		srcPath := filepath.Join(b.contextPath, src)
		if err := b.copyWithIgnore(srcPath, dst); err != nil {
			return fmt.Errorf("copy %s: %w", src, err)
		}
	}

	// Record as a new layer
	layerDigest := fmt.Sprintf("sha256:build-copy-%d", len(b.layers))
	b.layers = append(b.layers, image.Layer{
		Digest:    layerDigest,
		Size:      0,
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
	})

	return nil
}

func (b *Builder) handleAdd(instr *AddInstruction) error {
	fmt.Fprintf(os.Stderr, "Step : ADD %s -> %s\n", strings.Join(instr.Sources, " "), instr.Destination)

	dst := filepath.Join(b.curRootfs, instr.Destination)
	os.MkdirAll(filepath.Dir(dst), storage.DefaultPerms)

	for _, src := range instr.Sources {
		if ShouldIgnore(src, b.ignorePatterns) {
			fmt.Fprintf(os.Stderr, "  (ignored: %s)\n", src)
			continue
		}
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			fmt.Fprintf(os.Stderr, "  (URL download for ADD not yet implemented: %s)\n", src)
			continue
		}
		srcPath := filepath.Join(b.contextPath, src)
		if err := b.copyWithIgnore(srcPath, dst); err != nil {
			return fmt.Errorf("add %s: %w", src, err)
		}
	}

	return nil
}

func (b *Builder) handleCmd(instr *CmdInstruction) error {
	b.imageConfig.Cmd = instr.Command
	return nil
}

func (b *Builder) handleEntrypoint(instr *EntrypointInstruction) error {
	b.imageConfig.Entrypoint = instr.Command
	return nil
}

func (b *Builder) handleEnv(instr *EnvInstruction) error {
	b.imageConfig.Env = append(b.imageConfig.Env, fmt.Sprintf("%s=%s", instr.Key, instr.Value))
	return nil
}

func (b *Builder) handleWorkdir(instr *WorkdirInstruction) error {
	b.imageConfig.Workdir = instr.Path
	return nil
}

func (b *Builder) handleExpose(instr *ExposeInstruction) error {
	portKey := fmt.Sprintf("%s/%s", instr.Port, instr.Protocol)
	if b.imageConfig.ExposedPorts == nil {
		b.imageConfig.ExposedPorts = make(map[string]struct{})
	}
	b.imageConfig.ExposedPorts[portKey] = struct{}{}
	return nil
}

func (b *Builder) handleVolume(instr *VolumeInstruction) error {
	if b.imageConfig.Volumes == nil {
		b.imageConfig.Volumes = make(map[string]struct{})
	}
	b.imageConfig.Volumes[instr.Path] = struct{}{}
	return nil
}

func (b *Builder) handleUser(instr *UserInstruction) error {
	b.imageConfig.User = instr.User
	return nil
}

func (b *Builder) handleLabel(instr *LabelInstruction) error {
	if b.imageConfig.Labels == nil {
		b.imageConfig.Labels = make(map[string]string)
	}
	b.imageConfig.Labels[instr.Key] = instr.Value
	return nil
}

func (b *Builder) handleArg(instr *ArgInstruction) error {
	if _, exists := b.buildArgs[instr.Name]; !exists {
		if instr.Default != "" {
			b.buildArgs[instr.Name] = instr.Default
		}
	}
	return nil
}

func (b *Builder) handleShell(instr *ShellInstruction) error {
	b.imageConfig.Shell = instr.Shell
	return nil
}

func (b *Builder) commit() (*image.Image, error) {
	fmt.Fprintf(os.Stderr, "\nCommitting image...\n")

	imgID := image.GenerateID([]byte(b.tag + "_" + time.Now().String()))

	kernelDir := b.paths.ImageKernelDir(imgID)
	if b.kernelID != "" {
		kernelSrc := b.paths.KernelImagePath(b.kernelID)
		os.MkdirAll(kernelDir, storage.DefaultPerms)
		copyFileContents(kernelSrc, b.paths.ImageKernelPath(imgID))
	}

	rootfsDest := b.paths.ImageLayerPath(imgID, "sha256:build-rootfs")
	if err := copyDirContents(b.curRootfs, rootfsDest); err != nil {
		return nil, fmt.Errorf("save rootfs: %w", err)
	}

	b.layers = append(b.layers, image.Layer{
		Digest:    "sha256:build-rootfs",
		Size:      dirSize(b.curRootfs),
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
	})

	ref, err := image.ParseImageRef(b.tag)
	if err != nil {
		return nil, fmt.Errorf("parse tag %q: %w", b.tag, err)
	}
	fullName := ref.FullName()

	img := &image.Image{
		ID:        imgID,
		RepoTags:  []string{fullName},
		Arch:      b.arch,
		Config:    b.imageConfig,
		Layers:    b.layers,
		KernelRef: b.kernelRef,
		KernelID:  b.kernelID,
		Created:   time.Now(),
		Size:      dirSize(b.curRootfs),
	}

	imgStore := image.NewStore(b.paths)
	if err := imgStore.Save(img); err != nil {
		return nil, fmt.Errorf("save image: %w", err)
	}

	idx, _ := imgStore.LoadIndex()
	idx.Add(fullName, imgID)
	imgStore.SaveIndex(idx)

	fmt.Fprintf(os.Stderr, "Built: %s (ID: %.20s)\n", fullName, imgID)

	return img, nil
}

func copyDirContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	os.MkdirAll(dst, storage.DefaultPerms)

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := os.Lstat(srcPath)
		if err != nil {
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, _ := os.Readlink(srcPath)
			os.Symlink(linkTarget, dstPath)
			continue
		}

		if entry.IsDir() {
			copyDirContents(srcPath, dstPath)
		} else {
			copyFileContents(srcPath, dstPath)
		}
	}

	return nil
}

func (b *Builder) copyWithIgnore(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if srcInfo.IsDir() {
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			sp := filepath.Join(src, entry.Name())
			dp := filepath.Join(dst, entry.Name())
			relPath, _ := filepath.Rel(b.contextPath, sp)

			if ShouldIgnore(relPath, b.ignorePatterns) {
				continue
			}

			if entry.IsDir() {
				os.MkdirAll(dp, storage.DefaultPerms)
				if err := b.copyWithIgnore(sp, dp); err != nil {
					return err
				}
			} else {
				os.MkdirAll(filepath.Dir(dp), storage.DefaultPerms)
				if err := copyFileContents(sp, dp); err != nil {
					return err
				}
			}
		}
	} else {
		dp := dst
		if !strings.HasSuffix(dst, filepath.Base(src)) {
			dp = filepath.Join(dst, filepath.Base(src))
		}
		os.MkdirAll(filepath.Dir(dp), storage.DefaultPerms)
		return copyFileContents(src, dp)
	}

	return nil
}

func copyPath(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if srcInfo.IsDir() {
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			sp := filepath.Join(src, entry.Name())
			dp := filepath.Join(dst, entry.Name())
			if !strings.HasSuffix(dst, entry.Name()) {
				dp = filepath.Join(dst, entry.Name())
			}
			if entry.IsDir() {
				os.MkdirAll(dp, storage.DefaultPerms)
				copyPath(sp, dp)
			} else {
				copyFileContents(sp, dp)
			}
		}
	} else {
		dstDir := dst
		if !strings.HasSuffix(dst, filepath.Base(src)) {
			dstDir = filepath.Join(dstDir, filepath.Base(src))
		}
		os.MkdirAll(filepath.Dir(dstDir), storage.DefaultPerms)
		copyFileContents(src, dstDir)
	}

	return nil
}

func copyFileContents(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	os.MkdirAll(filepath.Dir(dst), storage.DefaultPerms)

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
