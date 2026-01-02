package main

import (
	"fmt"
	"os"

	"github.com/jmcdonald/codebak/internal/cli"
	"github.com/jmcdonald/codebak/internal/tui"
)

// version is set via ldflags at build time: -ldflags "-X main.version=x.y.z"
var version = "dev"

func main() {
	// Handle TUI mode (no args or ui/tui command)
	if len(os.Args) < 2 || os.Args[1] == "ui" || os.Args[1] == "tui" {
		if err := tui.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Use CLI for all other commands
	c := cli.New(version)
	c.Run()
}
