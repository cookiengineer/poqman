package cli

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

type Command struct {
	Name        string
	Description string
	Usage       string
	FlagSet     *flag.FlagSet
	Run         func(args []string) error
}

type Router struct {
	commands map[string]*Command
}

func NewRouter() *Router {
	return &Router{
		commands: make(map[string]*Command),
	}
}

func (r *Router) Register(cmd *Command) {
	if cmd.FlagSet == nil {
		cmd.FlagSet = flag.NewFlagSet(cmd.Name, flag.ExitOnError)
	}
	r.commands[cmd.Name] = cmd
}

func (r *Router) Dispatch(args []string) error {
	if len(args) < 1 {
		r.printUsage()
		return nil
	}

	cmdName := args[0]

	if cmdName == "help" || cmdName == "--help" || cmdName == "-h" {
		if len(args) > 1 {
			cmd, ok := r.commands[args[1]]
			if ok {
				r.printCommandHelp(cmd)
				return nil
			}
		}
		r.printUsage()
		return nil
	}

	cmd, ok := r.commands[cmdName]
	if !ok {
		r.printUsage()
		return fmt.Errorf("unknown command %q", cmdName)
	}

	if err := cmd.FlagSet.Parse(args[1:]); err != nil {
		return err
	}

	return cmd.Run(cmd.FlagSet.Args())
}

func (r *Router) printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: poqman <command> [options]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")

	var names []string
	for name := range r.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cmd := r.commands[name]
		fmt.Fprintf(os.Stderr, "  %-12s %s\n", name, cmd.Description)
	}
	fmt.Fprintf(os.Stderr, "\nRun 'poqman help <command>' for details.\n")
}

func (r *Router) printCommandHelp(cmd *Command) {
	fmt.Fprintf(os.Stderr, "Usage: poqman %s %s\n\n", cmd.Name, cmd.Usage)
	fmt.Fprintf(os.Stderr, "%s\n\n", cmd.Description)
	if cmd.FlagSet != nil {
		fmt.Fprintf(os.Stderr, "Options:\n")
		cmd.FlagSet.PrintDefaults()
	}
}

func FormatImageName(repo, tag string) string {
	shortRepo := repo
	if idx := strings.LastIndex(repo, "/"); idx >= 0 {
		shortRepo = repo[idx+1:]
	}
	return shortRepo
}
