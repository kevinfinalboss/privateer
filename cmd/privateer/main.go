package main

import (
	"os"

	"github.com/kevinfinalboss/privateer/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
