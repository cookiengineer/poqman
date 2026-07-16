package storage

import (
	"os"
	"path/filepath"
)

const (
	AppName          = "poqman"
	DefaultPerms     = 0o755
	FilePerms        = 0o644
	imagesDir        = "images"
	kernelsDir       = "kernels"
	containersDir    = "containers"
	networksDir      = "networks"
	tmpDir           = "tmp"
	layersDir        = "layers"
	kernelSubDir     = "kernel"
	indexFileName    = "index.json"
	manifestFileName = "manifest.json"
	configFileName   = "config.json"
	stateFileName    = "state.json"
	consoleFileName  = "console.log"
	pidFileName      = "pidfile"
	qmpSockName      = "qmp.sock"
	monitorSockName  = "monitor.sock"
	agentSockName    = "agent.sock"
	initBinaryName   = "init"
	agentBinaryName  = "poqman-agent"
	bzImageName      = "bzImage"
)

type Paths struct {
	Base       string
	Images     string
	Kernels    string
	Containers string
	Networks   string
	Tmp        string
}

func ResolvePaths() (*Paths, error) {
	base := baseDir()
	return &Paths{
		Base:       base,
		Images:     filepath.Join(base, imagesDir),
		Kernels:    filepath.Join(base, kernelsDir),
		Containers: filepath.Join(base, containersDir),
		Networks:   filepath.Join(base, networksDir),
		Tmp:        filepath.Join(base, tmpDir),
	}, nil
}

func (p *Paths) EnsureAll() error {
	for _, dir := range []string{
		p.Images,
		p.Kernels,
		p.Containers,
		p.Networks,
		p.Tmp,
	} {
		if err := os.MkdirAll(dir, DefaultPerms); err != nil {
			return err
		}
	}
	return nil
}

func (p *Paths) ImageIndexPath() string {
	return filepath.Join(p.Images, indexFileName)
}

func (p *Paths) ImagePath(id string) string {
	return filepath.Join(p.Images, id)
}

func (p *Paths) ImageManifestPath(id string) string {
	return filepath.Join(p.Images, id, manifestFileName)
}

func (p *Paths) ImageConfigPath(id string) string {
	return filepath.Join(p.Images, id, configFileName)
}

func (p *Paths) ImageLayersDir(id string) string {
	return filepath.Join(p.Images, id, layersDir)
}

func (p *Paths) ImageLayerPath(imageID, digest string) string {
	return filepath.Join(p.Images, imageID, layersDir, digest)
}

func (p *Paths) ImageKernelDir(id string) string {
	return filepath.Join(p.Images, id, kernelSubDir)
}

func (p *Paths) ImageKernelPath(id string) string {
	return filepath.Join(p.Images, id, kernelSubDir, bzImageName)
}

func (p *Paths) KernelIndexPath() string {
	return filepath.Join(p.Kernels, indexFileName)
}

func (p *Paths) KernelPath(id string) string {
	return filepath.Join(p.Kernels, id)
}

func (p *Paths) KernelImagePath(id string) string {
	return filepath.Join(p.Kernels, id, bzImageName)
}

func (p *Paths) KernelConfigPath(id string) string {
	return filepath.Join(p.Kernels, id, configFileName)
}

func (p *Paths) ContainerPath(id string) string {
	return filepath.Join(p.Containers, id)
}

func (p *Paths) ContainerConfigPath(id string) string {
	return filepath.Join(p.Containers, id, configFileName)
}

func (p *Paths) ContainerStatePath(id string) string {
	return filepath.Join(p.Containers, id, stateFileName)
}

func (p *Paths) ContainerRootfsPath(id string) string {
	return filepath.Join(p.Containers, id, "rootfs")
}

func (p *Paths) ContainerKernelDir(id string) string {
	return filepath.Join(p.Containers, id, kernelSubDir)
}

func (p *Paths) ContainerKernelPath(id string) string {
	return filepath.Join(p.Containers, id, kernelSubDir, bzImageName)
}

func (p *Paths) ContainerConsoleLogPath(id string) string {
	return filepath.Join(p.Containers, id, consoleFileName)
}

func (p *Paths) ContainerPIDFilePath(id string) string {
	return filepath.Join(p.Containers, id, pidFileName)
}

func (p *Paths) ContainerQMPSocketPath(id string) string {
	return filepath.Join(p.Containers, id, qmpSockName)
}

func (p *Paths) ContainerMonitorSocketPath(id string) string {
	return filepath.Join(p.Containers, id, monitorSockName)
}

func (p *Paths) ContainerAgentSocketPath(id string) string {
	return filepath.Join(p.Containers, id, agentSockName)
}

func (p *Paths) NetworkStatePath() string {
	return filepath.Join(p.Networks, stateFileName)
}

func baseDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, AppName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/tmp", AppName)
	}
	return filepath.Join(home, ".local", "share", AppName)
}
