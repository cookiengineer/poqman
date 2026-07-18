package runtime

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cookiengineer/poqman/pkg/container"
)

type QEMUConfig struct {
	Binary       string
	Kernel       string
	Initrd       string
	RootFS       string
	RootFSFormat string
	Machine      string
	CPU          string
	Memory       string
	SMP          int
	Append       string
	NetDevID     string
	TapName      string
	NetMAC       string
	QMPSocket    string
	MonitorSocket string
	AgentSocket  string
	PIDFile      string
	Console      string
	VolumeMounts []container.VolumeMount
	KVM          bool
	NoGraphic    bool
	Daemonize    bool
	UseInitrd    bool
}

func BuildArgs(cfg QEMUConfig) ([]string, error) {
	if cfg.Binary == "" {
		return nil, fmt.Errorf("QEMU binary not specified")
	}
	if cfg.Kernel == "" {
		return nil, fmt.Errorf("kernel image not specified")
	}

	args := []string{}

	if cfg.Machine != "" {
		args = append(args, "-M", cfg.Machine)
	}
	if cfg.CPU != "" {
		args = append(args, "-cpu", cfg.CPU)
	}
	if cfg.KVM {
		args = append(args, "-accel", "kvm")
	}
	if cfg.Memory == "" {
		cfg.Memory = "512M"
	}
	args = append(args, "-m", cfg.Memory)

	if cfg.SMP > 0 {
		args = append(args, "-smp", strconv.Itoa(cfg.SMP))
	} else {
		args = append(args, "-smp", "2")
	}

	if cfg.NoGraphic {
		args = append(args, "-nographic")
	} else {
		args = append(args, "-display", "none")
	}

	args = append(args, "-kernel", cfg.Kernel)

	if cfg.Initrd != "" {
		args = append(args, "-initrd", cfg.Initrd)
	}

	if cfg.Append != "" {
		args = append(args, "-append", cfg.Append)
	}

	if cfg.RootFS != "" {
		args = append(args,
			"-fsdev", fmt.Sprintf("local,id=rootfs,path=%s,security_model=mapped-xattr", cfg.RootFS),
			"-device", fmt.Sprintf("virtio-9p-pci,fsdev=rootfs,mount_tag=rootfs"),
		)
	}

	for i, vol := range cfg.VolumeMounts {
		mountTag := fmt.Sprintf("volume%d", i)
		roFlag := ""
		if vol.ReadOnly {
			roFlag = ",readonly=on"
		}
		args = append(args,
			"-fsdev", fmt.Sprintf("local,id=%s,path=%s,security_model=mapped-xattr%s", mountTag, vol.Source, roFlag),
			"-device", fmt.Sprintf("virtio-9p-pci,fsdev=%s,mount_tag=%s", mountTag, mountTag),
		)
	}

	if cfg.NetDevID != "" && cfg.TapName != "" {
		mac := cfg.NetMAC
		if mac == "" {
			mac = "52:54:00:12:34:56"
		}
		args = append(args,
			"-netdev", fmt.Sprintf("tap,id=%s,ifname=%s,script=no,downscript=no", cfg.NetDevID, cfg.TapName),
			"-device", fmt.Sprintf("virtio-net-pci,netdev=%s,mac=%s", cfg.NetDevID, mac),
		)
	}

	if cfg.QMPSocket != "" {
		args = append(args, "-qmp", fmt.Sprintf("unix:%s,server=on,wait=off", cfg.QMPSocket))
	}

	if cfg.MonitorSocket != "" {
		args = append(args, "-monitor", fmt.Sprintf("unix:%s,server=on,wait=off", cfg.MonitorSocket))
	}

	if cfg.AgentSocket != "" {
		args = append(args,
			"-chardev", fmt.Sprintf("socket,id=agent0,path=%s,server=on,wait=off", cfg.AgentSocket),
			"-device", "virtio-serial-pci",
			"-device", "virtserialport,chardev=agent0,name=poqman.agent",
		)
	}

	if cfg.PIDFile != "" {
		args = append(args, "-pidfile", cfg.PIDFile)
	}

	return args, nil
}

func BuildKernelAppend(cfg QEMUConfig) string {
	var parts []string

	if !cfg.UseInitrd {
		if cfg.RootFSFormat == "" || cfg.RootFSFormat == "9p" {
			parts = append(parts,
				"root=rootfs",
				"rootfstype=9p",
				"rootflags=trans=virtio,version=9p2000.L",
				"rw",
			)
		}
	}

	if cfg.Console != "" {
		parts = append(parts, "console="+cfg.Console)
	}

	parts = append(parts, "quiet")

	return strings.Join(parts, " ")
}

func BuildInitCmdline(cmd []string, hostname string, ip string, gateway string, dns string, volumes []container.VolumeMount) string {
	var parts []string

	if hostname != "" {
		parts = append(parts, "poqman.hostname="+escapeKernelArg(hostname))
	}
	if ip != "" {
		parts = append(parts, "poqman.ip="+escapeKernelArg(ip))
	}
	if gateway != "" {
		parts = append(parts, "poqman.gateway="+escapeKernelArg(gateway))
	}
	if dns != "" {
		parts = append(parts, "poqman.dns="+escapeKernelArg(dns))
	}

	for i, vol := range volumes {
		roFlag := "false"
		if vol.ReadOnly {
			roFlag = "true"
		}
		parts = append(parts,
			fmt.Sprintf("poqman.volume.%d.source=%s", i, escapeKernelArg(vol.Source)),
			fmt.Sprintf("poqman.volume.%d.target=%s", i, escapeKernelArg(vol.Target)),
			fmt.Sprintf("poqman.volume.%d.readonly=%s", i, roFlag),
		)
	}

	if len(cmd) > 0 {
		escapedCmd := make([]string, len(cmd))
		for i, c := range cmd {
			escapedCmd[i] = escapeKernelArg(c)
		}
		parts = append(parts, "poqman.cmd="+strings.Join(escapedCmd, " "))
	} else {
		parts = append(parts, "poqman.cmd=/sbin/init")
	}

	return strings.Join(parts, " ")
}

func escapeKernelArg(arg string) string {
	arg = strings.ReplaceAll(arg, "\"", "\\\"")
	if strings.ContainsAny(arg, " \t\n") {
		return "\"" + arg + "\""
	}
	return arg
}

func GenerateMAC(containerID string) string {
	if len(containerID) < 6 {
		containerID = containerID + "000000"
	}
	return fmt.Sprintf("52:54:00:%02x:%02x:%02x",
		containerID[0], containerID[1], containerID[2])
}
