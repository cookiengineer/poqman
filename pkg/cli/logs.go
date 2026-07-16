package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/cookiengineer/poqman/pkg/container"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func RegisterLogs(router *Router) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	follow := fs.Bool("f", false, "Follow log output")
	tail := fs.Int("tail", 0, "Number of lines to show from the end")

	router.Register(&Command{
		Name:        "logs",
		Description: "Fetch the logs of a container",
		Usage:       "[options] <container-id>",
		FlagSet:     fs,
		Run: func(args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("container ID required")
			}

			paths, _ := storage.ResolvePaths()
			paths.EnsureAll()

			store := container.NewStore(paths)
			_, err := store.Load(args[0])
			if err != nil {
				return fmt.Errorf("load container: %w", err)
			}

			logPath := paths.ContainerConsoleLogPath(args[0])

			if *follow && *tail > 0 {
				return tailFollowLog(logPath, *tail)
			}

			file, err := os.Open(logPath)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintln(os.Stderr, "No logs available")
					return nil
				}
				return fmt.Errorf("open log: %w", err)
			}
			defer file.Close()

			if *tail > 0 {
				return printLastLines(file, *tail)
			}

			_, err = io.Copy(os.Stdout, file)
			return err
		},
	})
}

func tailFollowLog(path string, tail int) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer file.Close()

	file.Seek(0, io.SeekEnd)

	buf := make([]byte, 4096)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
		}
		if err != nil {
			if err == io.EOF {
				continue
			}
			return err
		}
	}
}

func printLastLines(file *os.File, count int) error {
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	lines := splitLines(string(data))
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
