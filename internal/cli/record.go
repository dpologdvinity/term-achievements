package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dpologdvinity/term-achievements/internal/ach"
	"github.com/dpologdvinity/term-achievements/internal/parse"
	"github.com/dpologdvinity/term-achievements/internal/render"
	"github.com/dpologdvinity/term-achievements/internal/stats"
	"github.com/dpologdvinity/term-achievements/internal/store"
)

// recordLockBudget keeps the prompt from waiting on another shell: past it, the event is dropped.
const recordLockBudget = 5 * time.Millisecond

// runRecord records one finished command. It is the shell hook's hot path: it always returns 0
// and never writes to stderr, whatever goes wrong.
func runRecord(args []string, d Deps) (code int) {
	defer func() {
		if recover() != nil {
			code = 0
		}
	}()
	if d.Getenv("TERM_ACHIEVEMENTS_DISABLE") != "" {
		return 0
	}
	fs := flag.NewFlagSet("record", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	exit := fs.Int("exit", 0, "")
	ms := fs.Int64("ms", 0, "")
	started := fs.Int64("started", 0, "")
	prev := fs.String("prev", "", "")
	if fs.Parse(args) != nil {
		return 0
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if !set["exit"] {
		return 0
	}
	c := parse.Parse(strings.Join(fs.Args(), " "), *prev)
	if c.Name == "" {
		return 0
	}

	now := d.Now()
	var dur int64
	switch {
	case set["ms"]:
		dur = max(0, *ms)
	case set["started"]:
		dur = max(0, now.UnixMilli()-*started*1000)
	}
	cwd, err := d.Getwd()
	if err != nil {
		cwd = ""
	}
	e := stats.Event{Cmd: c, Exit: *exit, Ms: dur, Now: now, CwdType: stats.CwdType(cwd, d.Getenv("HOME"), d.TempDirs)}

	dir, err := store.Dir(d.Getenv)
	if err != nil {
		return 0
	}
	unlock, err := store.Lock(dir, false, recordLockBudget)
	if err != nil {
		return 0
	}
	defer unlock()
	s, _, err := store.Load(dir, now)
	if err != nil {
		return 0
	}
	stats.Apply(s, e)
	unlocked := ach.Evaluate(s, now)
	if store.Save(dir, s) != nil {
		return 0
	}
	fmt.Fprint(d.Stdout, render.Toasts(unlocked))
	return 0
}
