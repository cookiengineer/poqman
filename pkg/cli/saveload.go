package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func RegisterSave(router *Router) {
	fs := flag.NewFlagSet("save", flag.ExitOnError)
	output := fs.String("o", "", "Write to a file (default: <image>.tar.gz)")

	router.Register(&Command{
		Name:        "save",
		Description: "Save an image to a tar archive",
		Usage:       "[options] <image>",
		FlagSet:     fs,
		Run: func(args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("image name required")
			}

			paths, _ := storage.ResolvePaths()
			paths.EnsureAll()

			ref, err := image.ParseImageRef(args[0])
			if err != nil {
				return fmt.Errorf("parse image: %w", err)
			}

			store := image.NewStore(paths)
			img, err := store.Resolve(ref)
			if err != nil {
				return fmt.Errorf("resolve image: %w", err)
			}

			outPath := *output
			if outPath == "" {
				repoName := strings.ReplaceAll(ref.Repository, "/", "_")
				outPath = fmt.Sprintf("%s_%s.tar.gz", repoName, ref.Tag)
			}

			fmt.Fprintf(os.Stderr, "Saving %s → %s\n", ref.FullName(), outPath)
			if err := image.SaveImage(img, paths, outPath); err != nil {
				return fmt.Errorf("save: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Saved: %s\n", outPath)
			return nil
		},
	})
}

func RegisterLoad(router *Router) {
	fs := flag.NewFlagSet("load", flag.ExitOnError)
	input := fs.String("i", "", "Read from tar archive")

	router.Register(&Command{
		Name:        "load",
		Description: "Load an image from a tar archive",
		Usage:       "[options]",
		FlagSet:     fs,
		Run: func(args []string) error {
			if *input == "" {
				return fmt.Errorf("input file required (-i)")
			}

			paths, _ := storage.ResolvePaths()
			paths.EnsureAll()

			fmt.Fprintf(os.Stderr, "Loading from %s...\n", *input)
			img, err := image.LoadImage(paths, *input)
			if err != nil {
				return fmt.Errorf("load: %w", err)
			}

			extractDir := filepath.Join(paths.Tmp, "load-extract-"+time.Now().Format("20060102150405"))
			defer os.RemoveAll(extractDir)
			os.MkdirAll(extractDir, storage.DefaultPerms)

			img.ID = image.GenerateID([]byte(img.RepoTags[0] + "_" + time.Now().String()))
			img.Created = time.Now()

			store := image.NewStore(paths)
			if err := store.Save(img); err != nil {
				return fmt.Errorf("save loaded image: %w", err)
			}

			idx, _ := store.LoadIndex()
			for _, tag := range img.RepoTags {
				idx.Add(tag, img.ID)
				fmt.Fprintf(os.Stderr, "Loaded: %s\n", tag)
			}
			store.SaveIndex(idx)

			return nil
		},
	})
}
