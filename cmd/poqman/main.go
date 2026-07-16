package main

import (
	"fmt"
	"os"

	"github.com/cookiengineer/poqman/pkg/cli"
)

func main() {
	router := cli.NewRouter()

	cli.RegisterImages(router)
	cli.RegisterPs(router)
	cli.RegisterPull(router)
	cli.RegisterKernel(router)
	cli.RegisterRun(router)
	cli.RegisterStart(router)
	cli.RegisterStop(router)
	cli.RegisterLogs(router)
	cli.RegisterExec(router)
	cli.RegisterRm(router)
	cli.RegisterRmi(router)
	cli.RegisterInspect(router)
	cli.RegisterBuild(router)

	args := os.Args[1:]
	if len(args) == 0 {
		router.Dispatch([]string{"help"})
		return
	}

	if err := router.Dispatch(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
