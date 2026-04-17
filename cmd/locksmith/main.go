package main

import (
	"os"

	"github.com/lorem-dev/locksmith/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		cli.PrintError(err)
		os.Exit(1)
	}
}
