package cli

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cookiengineer/poqman/pkg/kernel"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func RegisterKernel(router *Router) {
	fs := flag.NewFlagSet("kernel", flag.ExitOnError)

	router.Register(&Command{
		Name:        "kernel",
		Description: "Manage kernel images",
		Usage:       "<pull|list|rm> [options]",
		FlagSet:     fs,
		Run: func(args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("kernel subcommand required: pull, list, or rm")
			}

			paths, err := storage.ResolvePaths()
			if err != nil {
				return fmt.Errorf("resolve storage paths: %w", err)
			}
			if err := paths.EnsureAll(); err != nil {
				return fmt.Errorf("ensure storage directories: %w", err)
			}

			subcmd := args[0]
			switch subcmd {
			case "pull":
				return runKernelPull(paths, args[1:])
			case "list":
				return runKernelList(paths)
			case "rm":
				return runKernelRemove(paths, args[1:])
			default:
				return fmt.Errorf("unknown kernel subcommand %q (expected: pull, list, rm)", subcmd)
			}
		},
	})
}

func runKernelPull(paths *storage.Paths, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("kernel reference required")
	}

	puller := kernel.NewPuller(paths)
	_, err := puller.Pull(args[0])
	return err
}

func runKernelList(paths *storage.Paths) error {
	store := kernel.NewStore(paths)
	kernels, err := store.List()
	if err != nil {
		return fmt.Errorf("list kernels: %w", err)
	}

	if len(kernels) == 0 {
		fmt.Fprintln(os.Stderr, "No kernels found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "DISTRO\tVERSION\tARCH\tKERNEL ID\tCREATED")
	for _, k := range kernels {
		shortID := k.ID
		if len(shortID) > 20 {
			shortID = shortID[:20]
		}
		created := k.Created.Format("2006-01-02 15:04")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			k.Distro, k.Version, k.Arch, shortID, created)
	}
	return w.Flush()
}

func runKernelRemove(paths *storage.Paths, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("kernel reference required")
	}

	req, err := kernel.ParseKernelRef(args[0])
	if err != nil {
		return fmt.Errorf("invalid kernel reference: %w", err)
	}

	store := kernel.NewStore(paths)
	k, err := store.Resolve(req)
	if err != nil {
		return fmt.Errorf("kernel not found: %w", err)
	}

	if err := store.Remove(k.ID); err != nil {
		return fmt.Errorf("remove kernel: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Removed kernel: %s\n", args[0])
	return nil
}
