package main

import (
	"os"
	"runtime/debug"
	"strings"

	"github.com/dpologdvinity/term-achievements/internal/cli"
)

var version = "dev"

func main() {
	if bi, ok := debug.ReadBuildInfo(); ok && version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		version = strings.TrimPrefix(bi.Main.Version, "v") // go install builds carry the module version
	}
	os.Exit(cli.Run(os.Args[1:], cli.RealDeps(version)))
}
