// Package cli wires the term-achievements commands to the stats, store and render packages.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/dpologdvinity/term-achievements/internal/hooks"
	"github.com/dpologdvinity/term-achievements/internal/render"
	"github.com/dpologdvinity/term-achievements/internal/stats"
	"github.com/dpologdvinity/term-achievements/internal/store"
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

const usage = `term-achievements - achievements for your shell

usage:
  term-achievements list                 show achievements and progress
  term-achievements wrapped [--year Y]   your year in the terminal
  term-achievements init <shell>         print the shell hook (zsh, bash, fish)
  term-achievements --version            print the version

Setup (zsh):  add  eval "$(term-achievements init zsh)"  to ~/.zshrc
Setup (bash): add  eval "$(term-achievements init bash)"  to ~/.bashrc
Setup (fish): add  term-achievements init fish | source  to ~/.config/fish/config.fish

Only counters are stored. Command lines and arguments never touch the disk.
`

func fail(d Deps, code int, format string, a ...any) int {
	fmt.Fprintf(d.Stderr, "term-achievements: "+format+"\n", a...)
	return code
}

// Run executes one term-achievements invocation and returns its exit code.
func Run(args []string, d Deps) int {
	cmd := "list"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "--version", "-V":
		fmt.Fprintf(d.Stdout, "term-achievements %s\n", d.Version)
	case "list":
		return withStats(d, func(s *stats.Stats) { fmt.Fprint(d.Stdout, render.List(s, d.Now().Location())) })
	case "wrapped":
		return runWrapped(args, d)
	case "init":
		if len(args) == 1 {
			if script, ok := hooks.Script(args[0]); ok {
				fmt.Fprint(d.Stdout, script)
				return 0
			}
		}
		return fail(d, 2, "init needs a shell: zsh, bash or fish")
	case "record":
		return runRecord(args, d)
	case "--help", "-h", "help":
		fmt.Fprint(d.Stdout, usage)
	default:
		return fail(d, 2, "unknown command %q (see term-achievements --help)", cmd)
	}
	return 0
}

// withStats loads the stats under a shared lock and passes them to show.
// A corrupt file is moved aside, announced once, and replaced by a fresh one.
func withStats(d Deps, show func(*stats.Stats)) int {
	dir, err := store.Dir(d.Getenv)
	if err != nil {
		return fail(d, 1, "%v", err)
	}
	unlock, err := store.Lock(dir, true, 2*time.Second)
	if err != nil {
		return fail(d, 1, "%v", err)
	}
	defer unlock()
	s, moved, err := store.Load(dir, d.Now())
	if err != nil {
		return fail(d, 1, "%v", err)
	}
	if moved != "" {
		fmt.Fprintf(d.Stderr, "term-achievements: stats file was unreadable; moved it to %s and started fresh\n", moved)
		if err := store.Save(dir, s); err != nil {
			return fail(d, 1, "%v", err)
		}
	}
	show(s)
	return 0
}

func runWrapped(args []string, d Deps) int {
	fs := flag.NewFlagSet("wrapped", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ys := fs.String("year", strconv.Itoa(d.Now().Year()), "")
	if err := fs.Parse(args); err != nil {
		return fail(d, 2, "%v", err)
	}
	year, err := strconv.Atoi(*ys)
	if err != nil || year < 1970 || year > 9999 || fs.NArg() != 0 {
		return fail(d, 2, "--year must be a year like 2026")
	}
	return withStats(d, func(s *stats.Stats) {
		card, ok := render.Wrapped(s, year)
		if !ok {
			card = fmt.Sprintf("no commands recorded in %d yet\n", year)
		}
		fmt.Fprint(d.Stdout, card)
	})
}
