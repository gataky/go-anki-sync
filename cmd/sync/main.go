package main

import (
	"os"

	"github.com/gataky/sync/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
