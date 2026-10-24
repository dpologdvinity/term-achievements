package main

import (
	"os"

	"github.com/dpologdvinity/termachievements/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.RealDeps(version)))
}
