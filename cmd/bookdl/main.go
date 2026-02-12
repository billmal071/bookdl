package main

import (
	"os"

	"github.com/billmal071/bookdl/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
