package render

import (
	"strings"
	"testing"
	"time"

	"github.com/dpologdvinity/term-achievements/internal/ach"
	"github.com/dpologdvinity/term-achievements/internal/stats"
)

var t0 = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

func TestToasts(t *testing.T) {
	if Toasts(nil) != "" {
		t.Fatal()
	}
	one := Toasts(ach.All[5:6])
	if one != "🏆 Achievement unlocked: Escaped Vim — exit vim 10 times\n" {
		t.Fatalf("%q", one)
	}
	four := Toasts(ach.All[0:4])
	want := "🏆 Achievement unlocked: First Steps — run your first command\n" +
		"🏆 Achievement unlocked: Warming Up — run 100 commands\n" +
		"🏆 …and 2 more (see term-achievements list)\n"
	if four != want {
		t.Fatalf("%q", four)
	}
	if three := Toasts(ach.All[0:3]); strings.Count(three, "\n") != 3 || strings.Contains(three, "more") {
		t.Fatal(three)
	}
}

func TestList(t *testing.T) {
	s := stats.NewStats()
	s.Counters["commands"] = 37
	s.Unlocked["first_steps"] = t0
	out := List(s, time.UTC)
	lines := strings.Split(out, "\n")
	if lines[0] != "term-achievements · 1/40 unlocked" || lines[1] != "" {
		t.Fatal(lines[:2])
	}
	if lines[2] != "✓ First Steps                run your first command                       2026-03-10" {
		t.Fatalf("%q", lines[2])
	}
	if lines[3] != "· Warming Up                 run 100 commands                             [###-------] 37/100" {
		t.Fatalf("%q", lines[3])
	}
	if len(lines) != 43 || lines[42] != "" { // 2 header + 40 rows + trailing newline
		t.Fatal(len(lines))
	}
}
