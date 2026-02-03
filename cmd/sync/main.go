package main

import (
	"os"

	"github.com/yourusername/sync/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
