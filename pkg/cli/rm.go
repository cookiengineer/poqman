package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/cookiengineer/poqman/pkg/container"
	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/network"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func RegisterRm(router *Router) {
	fs := flag.NewFlagSet("rm", flag.ExitOnError)
	force := fs.Bool("f", false, "Force removal (kill if running)")

	router.Register(&Command{
		Name:        "rm",
		Description: "Remove one or more containers",
		Usage:       "[options] <container-id>",
		FlagSet:     fs,
		Run: func(args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("container ID required")
			}

			paths, err := storage.ResolvePaths()
			if err != nil {
				return fmt.Errorf("resolve storage paths: %w", err)
			}
			paths.EnsureAll()

			store := container.NewStore(paths)

			for _, containerID := range args {
				c, err := store.Load(containerID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: container %q not found: %v\n", containerID, err)
					continue
				}

				if c.Status == container.StatusRunning {
					if !*force {
						return fmt.Errorf("container %q is running, use -f to force remove", c.ID[:12])
					}
					if err := forceKill(c, paths); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: force kill %q: %v\n", c.ID[:12], err)
					}
				}

				netManager := network.NewManager(paths)
				if c.IP != "" {
					netManager.ReleaseIP(c.ID)
				}

				if err := store.Remove(c.ID); err != nil {
					fmt.Fprintf(os.Stderr, "Error removing container %q: %v\n", c.ID[:12], err)
					continue
				}

				fmt.Fprintf(os.Stderr, "Removed container: %s\n", c.ID[:12])
			}

			return nil
		},
	})
}

func RegisterRmi(router *Router) {
	fs := flag.NewFlagSet("rmi", flag.ExitOnError)
	force := fs.Bool("f", false, "Force removal")

	router.Register(&Command{
		Name:        "rmi",
		Description: "Remove one or more images",
		Usage:       "[options] <image>",
		FlagSet:     fs,
		Run: func(args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("image name required")
			}

			paths, err := storage.ResolvePaths()
			if err != nil {
				return fmt.Errorf("resolve storage paths: %w", err)
			}
			paths.EnsureAll()

			imgStore := image.NewStore(paths)
			containerStore := container.NewStore(paths)

			for _, raw := range args {
				ref, err := image.ParseImageRef(raw)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: invalid image reference %q: %v\n", raw, err)
					continue
				}

				img, err := imgStore.Resolve(ref)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: image %q not found: %v\n", raw, err)
					continue
				}

				if !*force {
					containers, _ := containerStore.List()
					for _, c := range containers {
						if c.ImageID == img.ID && c.Status != container.StatusStopped {
							return fmt.Errorf("image %q is used by container %q (use -f to force)", raw, c.ID[:12])
						}
					}
				}

				if err := imgStore.Remove(img.ID); err != nil {
					return fmt.Errorf("remove image %q: %w", raw, err)
				}

				fmt.Fprintf(os.Stderr, "Removed image: %s\n", raw)
			}

			return nil
		},
	})
}

func forceKill(c *container.Container, paths *storage.Paths) error {
	proc, err := os.FindProcess(c.PID)
	if err == nil && proc != nil {
		proc.Kill()
	}

	c.Status = container.StatusStopped
	store := container.NewStore(paths)
	store.Save(c)

	return nil
}
