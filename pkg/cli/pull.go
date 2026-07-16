package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/registry"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func RegisterPull(router *Router) {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	platform := fs.String("platform", "", "Specify platform (e.g. linux/amd64, linux/arm64)")

	router.Register(&Command{
		Name:        "pull",
		Description: "Pull an image from a registry",
		Usage:       "[options] <image>",
		FlagSet:     fs,
		Run: func(args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("image name required")
			}

			ref, err := image.ParseImageRef(args[0])
			if err != nil {
				return fmt.Errorf("invalid image reference %q: %w", args[0], err)
			}

			plat, err := registry.ParsePlatform(*platform)
			if err != nil {
				return fmt.Errorf("invalid platform %q: %w", *platform, err)
			}

			paths, err := storage.ResolvePaths()
			if err != nil {
				return fmt.Errorf("resolve storage paths: %w", err)
			}
			if err := paths.EnsureAll(); err != nil {
				return fmt.Errorf("ensure storage directories: %w", err)
			}

			puller := registry.NewPuller(paths)

			_, err = puller.Pull(ref, plat)
			if err != nil {
				return fmt.Errorf("pull %s: %w", ref.FullName(), err)
			}

			fmt.Fprintf(os.Stderr, "Successfully pulled %s\n", ref.FullName())
			return nil
		},
	})
}
