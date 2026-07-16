package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cookiengineer/poqman/pkg/container"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func RegisterPs(router *Router) {
	fs := flag.NewFlagSet("ps", flag.ExitOnError)
	all := fs.Bool("a", false, "Show all containers (including stopped)")
	quiet := fs.Bool("q", false, "Show only container IDs")
	noHeading := fs.Bool("noheading", false, "Omit table header")

	router.Register(&Command{
		Name:        "ps",
		Description: "List containers",
		Usage:       "[options]",
		FlagSet:     fs,
		Run: func(args []string) error {
			paths, err := storage.ResolvePaths()
			if err != nil {
				return fmt.Errorf("resolve storage paths: %w", err)
			}
			if err := paths.EnsureAll(); err != nil {
				return fmt.Errorf("ensure storage directories: %w", err)
			}

			store := container.NewStore(paths)
			containers, err := store.List()
			if err != nil {
				return fmt.Errorf("list containers: %w", err)
			}

			filtered := make([]*container.Container, 0)
			for _, c := range containers {
				if *all || c.Status == container.StatusRunning || c.Status == container.StatusCreated {
					filtered = append(filtered, c)
				}
			}

			if len(filtered) == 0 {
				if !*quiet {
					fmt.Fprintln(os.Stderr, "No containers found.")
				}
				return nil
			}

			if *quiet {
				for _, c := range filtered {
					fmt.Println(c.ID)
				}
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			if !*noHeading {
				fmt.Fprintln(w, "CONTAINER ID\tIMAGE\tCOMMAND\tCREATED\tSTATUS\tPORTS\tNAMES")
			}
			for _, c := range filtered {
				shortID := c.ID
				if len(shortID) > 12 {
					shortID = shortID[:12]
				}
				imageName := c.ImageName
				if idx := strings.LastIndex(imageName, "/"); idx >= 0 {
					imageName = imageName[idx+1:]
				}
				cmd := strings.Join(c.Command, " ")
				created := formatDuration(time.Since(c.CreatedAt))
				status := formatStatus(c)
				ports := formatPorts(c)
				name := c.Name
				if name == "" {
					name = shortID
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					shortID, imageName, cmd, created, status, ports, name)
			}
			return w.Flush()
		},
	})
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}

func formatStatus(c *container.Container) string {
	switch c.Status {
	case container.StatusRunning:
		if !c.StartedAt.IsZero() {
			return "Up " + formatDuration(time.Since(c.StartedAt))
		}
		return "Up"
	case container.StatusStopped:
		if c.ExitCode != 0 {
			return fmt.Sprintf("Exited (%d)", c.ExitCode)
		}
		return "Exited"
	case container.StatusCreated:
		return "Created"
	case container.StatusFailed:
		return "Failed"
	}
	return string(c.Status)
}

func formatPorts(c *container.Container) string {
	if len(c.Ports) == 0 {
		return ""
	}
	var parts []string
	for _, p := range c.Ports {
		parts = append(parts, fmt.Sprintf("%d->%d/%s", p.HostPort, p.GuestPort, p.Protocol))
	}
	return strings.Join(parts, ", ")
}
