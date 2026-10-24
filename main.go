package main

import (
	"os"
	"runtime/debug"
	"strings"

	"github.com/dpologdvinity/term-achievements/internal/cli"
)

// version is the release version. GoReleaser sets it with -ldflags "-X main.version=...";
// plain `go build` leaves "dev", and main replaces that with the module version when it can.
var version = "dev"

// main runs the CLI with real dependencies and exits with its status code.
func main() {
	// `go install ...@vX.Y.Z` has no ldflags, but records the module version in the binary.
	if bi, ok := debug.ReadBuildInfo(); ok && version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		version = strings.TrimPrefix(bi.Main.Version, "v") // go install builds carry the module version
	}
	os.Exit(cli.Run(os.Args[1:], cli.RealDeps(version)))
}
