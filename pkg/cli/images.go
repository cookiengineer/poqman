package cli

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func RegisterImages(router *Router) {
	fs := flag.NewFlagSet("images", flag.ExitOnError)
	quiet := fs.Bool("q", false, "Show only image IDs")
	noHeading := fs.Bool("noheading", false, "Omit table header")

	router.Register(&Command{
		Name:        "images",
		Description: "List images in local storage",
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

			store := image.NewStore(paths)
			images, err := store.List()
			if err != nil {
				return fmt.Errorf("list images: %w", err)
			}

			if len(images) == 0 {
				if !*quiet {
					fmt.Fprintln(os.Stderr, "No images found.")
				}
				return nil
			}

			if *quiet {
				for _, img := range images {
					for _, tag := range img.RepoTags {
						fmt.Println(tag)
					}
				}
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			if !*noHeading {
				fmt.Fprintln(w, "REPOSITORY\tTAG\tIMAGE ID\tCREATED\tSIZE")
			}
			for _, img := range images {
				tag := "<none>"
				repo := "<none>"
				if len(img.RepoTags) > 0 {
					ref, _ := image.ParseImageRef(img.RepoTags[0])
					repo = ref.Repository
					tag = ref.Tag
				}
				shortID := img.ID
				if len(shortID) > 20 {
					shortID = shortID[:20]
				}
				created := img.Created.Format("2006-01-02 15:04")
				size := formatSize(img.Size)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", repo, tag, shortID, created, size)
			}
			return w.Flush()
		},
	})
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
