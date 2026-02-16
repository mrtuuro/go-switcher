package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mrtuuro/go-switcher/internal/app"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve working directory: %v\n", err)
		os.Exit(1)
	}

	cli, err := app.NewCLI(os.Stdout, os.Stderr, cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init switcher: %v\n", err)
		os.Exit(1)
	}

	if err := cli.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
