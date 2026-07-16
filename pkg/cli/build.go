package cli

import (
	"flag"
	"fmt"

	"github.com/cookiengineer/poqman/pkg/dockerfile"
)

func RegisterBuild(router *Router) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	tag := fs.String("t", "", "Name and optionally a tag for the image")
	fileFlag := fs.String("f", "Dockerfile", "Path to the Dockerfile")
	platform := fs.String("platform", "", "Set platform for the build (e.g., linux/amd64)")

	router.Register(&Command{
		Name:        "build",
		Description: "Build an image from a Dockerfile",
		Usage:       "[options] <path>",
		FlagSet:     fs,
		Run: func(args []string) error {
			if *tag == "" {
				return fmt.Errorf("image tag required (-t)")
			}

			contextPath := "."
			if len(args) > 0 {
				contextPath = args[0]
			}

			opts := dockerfile.BuildOptions{
				Tag:         *tag,
				ContextPath: contextPath,
				Dockerfile:  *fileFlag,
				Platform:    *platform,
			}

			_, err := dockerfile.Build(opts)
			if err != nil {
				return fmt.Errorf("build failed: %w", err)
			}

			fmt.Printf("Successfully built %s\n", *tag)
			return nil
		},
	})
}
