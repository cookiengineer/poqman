package runtime

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type Architecture struct {
	GoArch     string
	QEMUBinary string
	Machine    string
	CPU        string
	OCIPlatform string
}

var architectures = []Architecture{
	{GoArch: "amd64", QEMUBinary: "qemu-system-x86_64", Machine: "q35", CPU: "host", OCIPlatform: "linux/amd64"},
	{GoArch: "arm64", QEMUBinary: "qemu-system-aarch64", Machine: "virt", CPU: "host", OCIPlatform: "linux/arm64"},
	{GoArch: "arm", QEMUBinary: "qemu-system-arm", Machine: "virt", CPU: "host", OCIPlatform: "linux/arm/v7"},
	{GoArch: "riscv64", QEMUBinary: "qemu-system-riscv64", Machine: "virt", CPU: "host", OCIPlatform: "linux/riscv64"},
	{GoArch: "ppc64le", QEMUBinary: "qemu-system-ppc64", Machine: "pseries", CPU: "host", OCIPlatform: "linux/ppc64le"},
}

type Detector struct {
	hostArch string
	binary   string
	version  string
}

func NewDetector() *Detector {
	return &Detector{
		hostArch: runtime.GOARCH,
	}
}

func (d *Detector) FindQEMU() (string, error) {
	if d.binary != "" {
		return d.binary, nil
	}

	for _, arch := range architectures {
		if arch.GoArch == d.hostArch {
			path, err := exec.LookPath(arch.QEMUBinary)
			if err == nil {
				d.binary = path
				return path, nil
			}
			return "", fmt.Errorf("QEMU not found: %s not in PATH (install qemu-system-%s)", arch.QEMUBinary, strings.TrimPrefix(arch.QEMUBinary, "qemu-system-"))
		}
	}

	return "", fmt.Errorf("unsupported host architecture: %s", d.hostArch)
}

func (d *Detector) GetArchSpec(goarch string) (Architecture, error) {
	if goarch == "" {
		goarch = d.hostArch
	}
	for _, arch := range architectures {
		if arch.GoArch == goarch {
			return arch, nil
		}
	}
	return Architecture{}, fmt.Errorf("unsupported architecture: %s", goarch)
}

func (d *Detector) HostArchSpec() (Architecture, error) {
	return d.GetArchSpec(d.hostArch)
}

func (d *Detector) BinaryForArch(goarch string) (string, error) {
	if goarch == "" || goarch == d.hostArch {
		return d.FindQEMU()
	}
	for _, arch := range architectures {
		if arch.GoArch == goarch {
			return exec.LookPath(arch.QEMUBinary)
		}
	}
	return "", fmt.Errorf("unsupported architecture: %s", goarch)
}

func (d *Detector) Version() (string, error) {
	if d.version != "" {
		return d.version, nil
	}
	binary, err := d.FindQEMU()
	if err != nil {
		return "", err
	}
	out, err := exec.Command(binary, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("get qemu version: %w", err)
	}
	d.version = strings.TrimSpace(string(out))
	return d.version, nil
}

func DefaultConsoleDevice(arch string) string {
	switch arch {
	case "arm64", "arm", "riscv64":
		return "ttyAMA0"
	default:
		return "ttyS0"
	}
}
