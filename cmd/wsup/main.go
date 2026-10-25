package main

import (
	"os"

	"github.com/brendenchu/wsup/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
