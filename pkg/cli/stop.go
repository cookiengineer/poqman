package cli

import (
	"flag"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/cookiengineer/poqman/pkg/container"
	"github.com/cookiengineer/poqman/pkg/network"
	"github.com/cookiengineer/poqman/pkg/runtime"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func RegisterStop(router *Router) {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	timeout := fs.Int("t", 30, "Seconds to wait before force kill")

	router.Register(&Command{
		Name:        "stop",
		Description: "Stop one or more running containers",
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

			if c.Status != container.StatusRunning {
				fmt.Fprintf(os.Stderr, "Container %q is not running (status: %s)\n", c.ID[:12], c.Status)
				return nil
			}

			qmpSocket := paths.ContainerQMPSocketPath(c.ID)

			qmp, err := runtime.QMPConnect(qmpSocket)
			if err != nil {
				fmt.Fprintf(os.Stderr, "QMP connect failed, force killing: %v\n", err)
				forceKillContainer(c, paths, store)
				return nil
			}
			defer qmp.Close()

			fmt.Fprintf(os.Stderr, "Stopping container %s...\n", c.ID[:12])

			if err := qmp.PowerDown(); err != nil {
				fmt.Fprintf(os.Stderr, "QMP powerdown failed: %v\n", err)
			}

			waitForContainerExit(c, time.Duration(*timeout)*time.Second)

			netManager := network.NewManager(paths)
			for _, port := range c.Ports {
				netManager.RemovePortForward(port.HostPort, port.Protocol)
			}
			if c.IP != "" {
				netManager.ReleaseIP(c.ID)
			}

			c.Status = container.StatusStopped
			c.FinishedAt = time.Now()
			store.Save(c)

			fmt.Fprintf(os.Stderr, "Container %s stopped\n", c.ID[:12])

			return nil
		},
	})
}

func forceKillContainer(c *container.Container, paths *storage.Paths, store *container.Store) {
	proc, _ := os.FindProcess(c.PID)
	if proc != nil {
		proc.Kill()
	}

	netManager := network.NewManager(paths)
	for _, port := range c.Ports {
		netManager.RemovePortForward(port.HostPort, port.Protocol)
	}
	if c.IP != "" {
		netManager.ReleaseIP(c.ID)
	}

	c.Status = container.StatusStopped
	c.FinishedAt = time.Now()
	store.Save(c)
}

func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func waitForContainerExit(c *container.Container, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isProcessAlive(c.PID) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	proc, _ := os.FindProcess(c.PID)
	if proc != nil {
		proc.Kill()
	}
}
