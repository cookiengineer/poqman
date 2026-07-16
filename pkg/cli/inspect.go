package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"github.com/cookiengineer/poqman/pkg/container"
	"github.com/cookiengineer/poqman/pkg/image"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func RegisterInspect(router *Router) {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)

	router.Register(&Command{
		Name:        "inspect",
		Description: "Display detailed information on containers or images",
		Usage:       "[options] <container-id|image>",
		FlagSet:     fs,
		Run: func(args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("container ID or image name required")
			}

			paths, err := storage.ResolvePaths()
			if err != nil {
				return fmt.Errorf("resolve storage paths: %w", err)
			}
			paths.EnsureAll()

			target := args[0]

			containerStore := container.NewStore(paths)
			c, cErr := containerStore.Load(target)
			if cErr == nil {
				return printJSON(containerInspectResult(c, paths))
			}

			ref, refErr := image.ParseImageRef(target)
			if refErr == nil {
				imgStore := image.NewStore(paths)
				img, imgErr := imgStore.Resolve(ref)
				if imgErr == nil {
					return printJSON(imageInspectResult(img, paths))
				}
			}

			return fmt.Errorf("no container or image found for %q", target)
		},
	})
}

type ContainerInspect struct {
	ID         string                   `json:"id"`
	Name       string                   `json:"name,omitempty"`
	ImageID    string                   `json:"imageId"`
	ImageName  string                   `json:"imageName"`
	Command    []string                 `json:"command"`
	Status     string                   `json:"status"`
	PID        int                      `json:"pid,omitempty"`
	IP         string                   `json:"ip,omitempty"`
	Ports      []container.PortMapping  `json:"ports,omitempty"`
	Volumes    []container.VolumeMount  `json:"volumes,omitempty"`
	CreatedAt  string                   `json:"createdAt"`
	StartedAt  string                   `json:"startedAt"`
	FinishedAt string                   `json:"finishedAt"`
	ExitCode   int                      `json:"exitCode"`
}

type ImageInspect struct {
	ID        string              `json:"id"`
	RepoTags  []string            `json:"repoTags"`
	Arch      string              `json:"arch"`
	KernelRef string              `json:"kernelRef,omitempty"`
	Config    image.ImageConfig   `json:"config"`
	Layers    []image.Layer       `json:"layers"`
	Created   string              `json:"created"`
	Size      int64               `json:"size"`
}

func containerInspectResult(c *container.Container, paths *storage.Paths) *ContainerInspect {
	ci := &ContainerInspect{
		ID:         c.ID,
		Name:       c.Name,
		ImageID:    c.ImageID,
		ImageName:  c.ImageName,
		Command:    c.Command,
		Status:     string(c.Status),
		PID:        c.PID,
		IP:         c.IP,
		Ports:      c.Ports,
		Volumes:    c.Volumes,
		CreatedAt:  formatTime(c.CreatedAt),
		StartedAt:  formatTime(c.StartedAt),
		FinishedAt: formatTime(c.FinishedAt),
		ExitCode:   c.ExitCode,
	}
	return ci
}

func imageInspectResult(img *image.Image, paths *storage.Paths) *ImageInspect {
	ii := &ImageInspect{
		ID:        img.ID,
		RepoTags:  img.RepoTags,
		Arch:      img.Arch,
		KernelRef: img.KernelRef,
		Config:    img.Config,
		Layers:    img.Layers,
		Created:   formatTime(img.Created),
		Size:      img.Size,
	}
	return ii
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
