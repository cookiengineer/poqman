package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cookiengineer/poqman/pkg/container"
	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/kernel"
	"github.com/cookiengineer/poqman/pkg/network"
	"github.com/cookiengineer/poqman/pkg/registry"
	"github.com/cookiengineer/poqman/pkg/runtime"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func RegisterRun(router *Router) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	name := fs.String("name", "", "Container name")
	detach := fs.Bool("d", false, "Run container in background")
	interactive := fs.Bool("i", false, "Keep stdin open")
	tty := fs.Bool("t", false, "Allocate a pseudo-TTY")
	port := fs.String("p", "", "Publish port (e.g., 8080:80)")
	memory := fs.String("m", "512M", "Memory limit")
	cpus := fs.Int("cpus", 2, "Number of CPUs")
	platformFlag := fs.String("platform", "", "Platform (e.g., linux/amd64)")
	volumeStr := fs.String("v", "", "Bind mount volume (e.g., /host:/guest)")
	rmFlag := fs.Bool("rm", false, "Remove container on exit")

	router.Register(&Command{
		Name:        "run",
		Description: "Run a command in a new container",
		Usage:       "[options] <image> [command]",
		FlagSet:     fs,
		Run: func(args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("image name required")
			}

			ref, err := image.ParseImageRef(args[0])
			if err != nil {
				return fmt.Errorf("invalid image reference %q: %w", args[0], err)
			}

			cmdArgs := args[1:]

			paths, err := storage.ResolvePaths()
			if err != nil {
				return fmt.Errorf("resolve paths: %w", err)
			}
			if err := paths.EnsureAll(); err != nil {
				return fmt.Errorf("ensure storage: %w", err)
			}

			plat := registry.HostPlatform()
			if *platformFlag != "" {
				plat, err = registry.ParsePlatform(*platformFlag)
				if err != nil {
					return fmt.Errorf("invalid platform: %w", err)
				}
			}

			puller := registry.NewPuller(paths)
			img, err := puller.Pull(ref, plat)
			if err != nil {
				return fmt.Errorf("pull image: %w", err)
			}

			netManager := network.NewManager(paths)
			if err := netManager.Initialize(); err != nil {
				return fmt.Errorf("initialize network: %w", err)
			}

			detect := runtime.NewDetector()

			archSpec, err := detect.GetArchSpec(img.Arch)
			if err != nil {
				return fmt.Errorf("unsupported image architecture %s: %w", img.Arch, err)
			}

			binary, err := detect.FindQEMU()
			if err != nil {
				return fmt.Errorf("find QEMU: %w", err)
			}

			containerID := container.GenerateID()
			if *name != "" {
				containerID = *name
			}

			containerDir := paths.ContainerPath(containerID)
			if err := os.MkdirAll(containerDir, storage.DefaultPerms); err != nil {
				return fmt.Errorf("create container dir: %w", err)
			}

			rootfsPath, err := storage.AssembleRootfs(img.ID, containerID, paths)
			if err != nil {
				return fmt.Errorf("assemble rootfs: %w", err)
			}

			initBinary := InitBinary(archSpec.GoArch)
			if err := storage.InjectInit(rootfsPath, initBinary); err != nil {
				return fmt.Errorf("inject init: %w", err)
			}

			agentBinary := AgentBinary(archSpec.GoArch)
			if err := storage.InjectAgent(rootfsPath, agentBinary); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: inject agent: %v\n", err)
			}

			kernelSrc := paths.ImageKernelPath(img.ID)
			if _, err := os.Stat(kernelSrc); err == nil {
				kernelDst := paths.ContainerKernelDir(containerID)
				if err := storage.CopyKernel(kernelSrc, kernelDst); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: copy kernel: %v\n", err)
				}
			} else {
				fmt.Fprintf(os.Stderr,
					"Warning: no kernel bundled with image %s.\n"+
						"Build with KERNEL directive or use --kernel flag.\n", img.RepoTags[0])
			}

			c := &container.Container{
				ID:        containerID,
				ImageID:   img.ID,
				ImageName: ref.FullName(),
				Command:   cmdArgs,
				Status:    container.StatusCreated,
				Name:      *name,
				CreatedAt: time.Now(),
			}

			var tapName, ipAddr string
			hasNetwork := true

			tapName, ipAddr, err = netManager.CreateTap(containerID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: network setup failed: %v\n", err)
				hasNetwork = false
			}

			var ports []container.PortMapping
			if *port != "" {
				p, err := parsePortMapping(*port)
				if err == nil && hasNetwork {
					for _, pm := range p {
						netManager.AddPortForward("", pm.HostPort, ipAddr, pm.GuestPort, pm.Protocol)
					}
					ports = p
				}
			}

			var volumes []container.VolumeMount
			if *volumeStr != "" {
				volumes = parseVolumeMounts(*volumeStr)
			}

			console := runtime.DefaultConsoleDevice(archSpec.GoArch)

			kernelPath := paths.ContainerKernelPath(containerID)
			if _, err := os.Stat(kernelPath); os.IsNotExist(err) {
				return fmt.Errorf("no kernel found. Build image with KERNEL directive first")
			}

			initrdPath := paths.ContainerInitrdPath(containerID)
			var useInitrd bool
			if img.KernelID != "" && kernel.HasNinePModules(paths, img.KernelID) {
				initBinary := InitBinary(archSpec.GoArch)
				if err := kernel.BuildInitrd(paths, img.KernelID, initBinary, initrdPath); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: initrd build failed: %v\n", err)
				} else {
					useInitrd = true
				}
			}

			consoleLog := paths.ContainerConsoleLogPath(containerID)

			qemuCfg := runtime.QEMUConfig{
				Binary:        binary,
				Kernel:        kernelPath,
				RootFS:        rootfsPath,
				RootFSFormat:  "9p",
				Machine:       archSpec.Machine,
				CPU:           archSpec.CPU,
				Memory:        *memory,
				SMP:           *cpus,
				Console:       console,
				NoGraphic:     true,
				QMPSocket:     paths.ContainerQMPSocketPath(containerID),
				MonitorSocket: paths.ContainerMonitorSocketPath(containerID),
				AgentSocket:   paths.ContainerAgentSocketPath(containerID),
				PIDFile:       paths.ContainerPIDFilePath(containerID),
				VolumeMounts:  volumes,
				UseInitrd:     useInitrd,
				Initrd:        initrdPath,
			}

			if hasNetwork && tapName != "" {
				qemuCfg.NetDevID = "net0"
				qemuCfg.TapName = tapName
				qemuCfg.NetMAC = runtime.GenerateMAC(containerID)
			}

			appendParts := runtime.BuildKernelAppend(qemuCfg)
			initCmdline := runtime.BuildInitCmdline(cmdArgs, containerID,
				ipAddr, netManager.Gateway, "1.1.1.1", volumes)
			qemuCfg.Append = appendParts + " " + initCmdline

			qemuArgs, err := runtime.BuildArgs(qemuCfg)
			if err != nil {
				return fmt.Errorf("build QEMU args: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Container %s starting...\n", containerID[:12])

			var proc *runtime.Process
			var ts *runtime.TerminalState

			if *interactive && *tty {
				ts, _ = runtime.MakeRawTerminal()
				proc, err = runtime.StartProcess(binary, qemuArgs, qemuCfg.PIDFile)
			} else if *detach || !(*interactive && *tty) {
				proc, err = runtime.StartProcessDetached(binary, qemuArgs, qemuCfg.PIDFile, consoleLog)
			} else {
				proc, err = runtime.StartProcess(binary, qemuArgs, qemuCfg.PIDFile)
			}

			if err != nil {
				if hasNetwork {
					netManager.DestroyTap(tapName)
				}
				return fmt.Errorf("start QEMU: %w", err)
			}

			c.Status = container.StatusRunning
			c.PID = proc.PID
			c.IP = ipAddr
			c.Ports = ports
			c.Volumes = volumes
			c.StartedAt = time.Now()

			store := container.NewStore(paths)
			if err := store.Create(c); err != nil {
				return fmt.Errorf("save container: %w", err)
			}

			if *detach {
				fmt.Println(containerID[:12])
			} else if !(*interactive && *tty) {
				fmt.Println(containerID[:12])
			}

			if *rmFlag {
				defer func() {
					proc.WaitWithTimeout(5 * time.Second)
					netManager.DestroyTap(tapName)
					netManager.ReleaseIP(containerID)
					store.Remove(containerID)
				}()
			}

			if *interactive && *tty {
				proc.Wait()
				if ts != nil {
					ts.Restore()
				}
				c.Status = container.StatusStopped
				c.FinishedAt = time.Now()
				store.Save(c)

				if *rmFlag {
					netManager.DestroyTap(tapName)
					netManager.ReleaseIP(containerID)
					store.Remove(containerID)
				}
			}

			return nil
		},
	})
}

func parsePortMapping(raw string) ([]container.PortMapping, error) {
	var result []container.PortMapping

	parts := strings.Split(raw, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		proto := "tcp"

		if idx := strings.Index(part, "/"); idx > 0 {
			proto = part[idx+1:]
			part = part[:idx]
		}

		pair := strings.Split(part, ":")
		if len(pair) != 2 {
			return nil, fmt.Errorf("invalid port mapping: %s", part)
		}

		var hostPort, guestPort int
		fmt.Sscanf(pair[0], "%d", &hostPort)
		fmt.Sscanf(pair[1], "%d", &guestPort)

		result = append(result, container.PortMapping{
			HostPort:  hostPort,
			GuestPort: guestPort,
			Protocol:  proto,
		})
	}

	return result, nil
}

func parseVolumeMounts(raw string) []container.VolumeMount {
	var result []container.VolumeMount
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		readOnly := false
		if strings.HasSuffix(part, ":ro") {
			readOnly = true
			part = part[:len(part)-3]
		}
		pair := strings.SplitN(part, ":", 2)
		if len(pair) != 2 {
			continue
		}
		result = append(result, container.VolumeMount{
			Source:   pair[0],
			Target:   pair[1],
			ReadOnly: readOnly,
		})
	}
	return result
}
