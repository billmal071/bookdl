package main

import (
	"os"

	"github.com/williams/bookdl/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
