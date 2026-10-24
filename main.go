package main

import (
	"os"

	"github.com/dpologdvinity/term-achievements/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.RealDeps(version)))
}
