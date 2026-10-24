// Package cli wires the termachievements commands to the stats, store and render packages.
package cli

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Deps holds everything Run needs from the outside world, so tests can inject it.
type Deps struct {
	Version        string
	Now            func() time.Time
	Getenv         func(string) string
	Getwd          func() (string, error)
	TempDirs       []string // production: os.TempDir() and "/tmp" (deduplicated)
	Stdout, Stderr io.Writer
}

// RealDeps returns the production dependencies.
func RealDeps(version string) Deps {
	tmps := []string{os.TempDir()}
	if os.TempDir() != "/tmp" {
		tmps = append(tmps, "/tmp")
	}
	return Deps{
		Version:  version,
		Now:      time.Now,
		Getenv:   os.Getenv,
		Getwd:    os.Getwd,
		TempDirs: tmps,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
	}
}

const usage = `termachievements - achievements for your shell

usage:
  termachievements list                 show achievements and progress
  termachievements wrapped [--year Y]   your year in the terminal
  termachievements init <shell>         print the shell hook (zsh, bash, fish)
  termachievements --version            print the version

Setup (zsh):  add  eval "$(termachievements init zsh)"  to ~/.zshrc
Setup (bash): add  eval "$(termachievements init bash)"  to ~/.bashrc
Setup (fish): add  termachievements init fish | source  to ~/.config/fish/config.fish

Only counters are stored. Command lines and arguments never touch the disk.
`

func fail(d Deps, code int, format string, a ...any) int {
	fmt.Fprintf(d.Stderr, "termachievements: "+format+"\n", a...)
	return code
}

// Run executes one termachievements invocation and returns its exit code.
func Run(args []string, d Deps) int {
	cmd := "list"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "--version", "-V":
		fmt.Fprintf(d.Stdout, "termachievements %s\n", d.Version)
	case "record":
		return runRecord(args, d)
	case "--help", "-h", "help":
		fmt.Fprint(d.Stdout, usage)
	default:
		return fail(d, 2, "unknown command %q (see termachievements --help)", cmd)
	}
	return 0
}
