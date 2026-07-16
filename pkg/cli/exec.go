package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/cookiengineer/poqman/pkg/container"
	"github.com/cookiengineer/poqman/pkg/runtime"
	"github.com/cookiengineer/poqman/pkg/storage"
)

func RegisterExec(router *Router) {
	fs := flag.NewFlagSet("exec", flag.ExitOnError)
	workdir := fs.String("workdir", "", "Working directory inside the container")
	envArgs := strSliceFlag{}

	router.Register(&Command{
		Name:        "exec",
		Description: "Execute a command in a running container",
		Usage:       "[options] <container-id> <command> [args...]",
		FlagSet:     fs,
		Run: func(args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("container ID and command required")
			}

			containerID := args[0]
			cmdArgs := args[1:]

			paths, err := storage.ResolvePaths()
			if err != nil {
				return fmt.Errorf("resolve storage paths: %w", err)
			}
			paths.EnsureAll()

			store := container.NewStore(paths)
			c, err := store.Load(containerID)
			if err != nil {
				return fmt.Errorf("load container: %w", err)
			}

			if c.Status != container.StatusRunning {
				return fmt.Errorf("container %q is not running (status: %s)", containerID[:12], c.Status)
			}

			agentSocket := paths.ContainerAgentSocketPath(c.ID)

			client, err := runtime.AgentConnect(agentSocket)
			if err != nil {
				return fmt.Errorf("connect to agent: %w\nMake sure poqman-agent is running inside the container", err)
			}
			defer client.Close()

			if err := client.Ping(); err != nil {
				return fmt.Errorf("agent not responding: %w", err)
			}

			cwd := *workdir
			var env []string
			for _, e := range envArgs {
				env = append(env, e)
			}

			exitCode, stdout, stderr, err := client.Execute(cmdArgs, env, cwd)
			if err != nil {
				return fmt.Errorf("exec failed: %w", err)
			}

			os.Stdout.WriteString(stdout)
			os.Stderr.WriteString(stderr)

			if exitCode != 0 {
				return fmt.Errorf("command exited with code %d", exitCode)
			}

			return nil
		},
	})
}

type strSliceFlag []string

func (s *strSliceFlag) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *strSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}
