package cli

import (
	"flag"
	"fmt"
	"runtime"

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

			goarch := runtime.GOARCH
			if *platform != "" {
				goarch = platformToGoarch(*platform)
			}

			opts := dockerfile.BuildOptions{
				Tag:         *tag,
				ContextPath: contextPath,
				Dockerfile:  *fileFlag,
				Platform:    *platform,
				InitBinary:  InitBinary(goarch),
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

func platformToGoarch(platform string) string {
	switch platform {
	case "linux/amd64":
		return "amd64"
	case "linux/arm64":
		return "arm64"
	case "linux/arm", "linux/arm/v7":
		return "arm"
	case "linux/riscv64":
		return "riscv64"
	case "linux/ppc64le":
		return "ppc64le"
	default:
		return "amd64"
	}
}
