package main

import (
	"fmt"
	"os"

	"github.com/kevinfinalboss/privateer/internal/cli"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("Privateer %s\n", Version)
		fmt.Printf("Build: %s\n", BuildTime)
		fmt.Printf("Commit: %s\n", GitCommit)
		return
	}

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
