package cli

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cookiengineer/poqman/pkg/container"
	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/network"
	"github.com/cookiengineer/poqman/pkg/runtime"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func RegisterStart(router *Router) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	attach := fs.Bool("a", false, "Attach to container's console")

	router.Register(&Command{
		Name:        "start",
		Description: "Start one or more stopped containers",
		Usage:       "[options] <container-id>",
		FlagSet:     fs,
		Run: func(args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("container ID required")
			}

			paths, _ := storage.ResolvePaths()
			paths.EnsureAll()

			store := container.NewStore(paths)
			c, err := store.Load(args[0])
			if err != nil {
				return fmt.Errorf("load container: %w", err)
			}

			if c.Status != container.StatusStopped && c.Status != container.StatusFailed {
				return fmt.Errorf("container %q is not stopped (status: %s)", c.ID[:12], c.Status)
			}

			netManager := network.NewManager(paths)
			netManager.Initialize()

			detect := runtime.NewDetector()
			binary, err := detect.FindQEMU()
			if err != nil {
				return fmt.Errorf("find QEMU: %w", err)
			}

			imgStore := image.NewStore(paths)
			img, err := imgStore.Get(c.ImageID)
			if err != nil {
				return fmt.Errorf("find image: %w", err)
			}

			archSpec, _ := detect.GetArchSpec(img.Arch)

			tapName, ipAddr, err := netManager.CreateTap(c.ID)
			if err != nil {
				return fmt.Errorf("network: %w", err)
			}

			for _, p := range c.Ports {
				netManager.AddPortForward("", p.HostPort, ipAddr, p.GuestPort, p.Protocol)
			}

			console := runtime.DefaultConsoleDevice(archSpec.GoArch)
			consoleLog := paths.ContainerConsoleLogPath(c.ID)

			qemuCfg := runtime.QEMUConfig{
				Binary:        binary,
				Kernel:        paths.ContainerKernelPath(c.ID),
				RootFS:        paths.ContainerRootfsPath(c.ID),
				RootFSFormat:  "9p",
				Machine:       archSpec.Machine,
				CPU:           archSpec.CPU,
				Memory:        "512M",
				SMP:           2,
				Console:       console,
				NoGraphic:     true,
				QMPSocket:     paths.ContainerQMPSocketPath(c.ID),
				MonitorSocket: paths.ContainerMonitorSocketPath(c.ID),
				AgentSocket:   paths.ContainerAgentSocketPath(c.ID),
				PIDFile:       paths.ContainerPIDFilePath(c.ID),
				VolumeMounts:  c.Volumes,
				NetDevID:      "net0",
				TapName:       tapName,
				NetMAC:        runtime.GenerateMAC(c.ID),
			}

			appendParts := runtime.BuildKernelAppend(qemuCfg)
			initCmdline := runtime.BuildInitCmdline(c.Command, c.ID,
				ipAddr, netManager.Gateway, "1.1.1.1", c.Volumes)
			qemuCfg.Append = appendParts + " " + initCmdline

			qemuArgs, err := runtime.BuildArgs(qemuCfg)
			if err != nil {
				return fmt.Errorf("build QEMU args: %w", err)
			}

			var proc *runtime.Process
			if *attach {
				proc, err = runtime.StartProcess(binary, qemuArgs, qemuCfg.PIDFile)
			} else {
				proc, err = runtime.StartProcessDetached(binary, qemuArgs, qemuCfg.PIDFile, consoleLog)
			}

			if err != nil {
				netManager.DestroyTap(tapName)
				return fmt.Errorf("start QEMU: %w", err)
			}

			c.Status = container.StatusRunning
			c.PID = proc.PID
			c.IP = ipAddr
			c.StartedAt = time.Now()
			store.Save(c)

			fmt.Fprintf(os.Stderr, "Container %s started\n", c.ID[:12])

			if *attach {
				proc.Wait()
				c.Status = container.StatusStopped
				c.FinishedAt = time.Now()
				store.Save(c)
			}

			return nil
		},
	})
}

