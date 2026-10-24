package render

import (
	"strings"
	"testing"
	"time"

	"github.com/dpologdvinity/termachievements/internal/stats"
)

func TestWrappedFull(t *testing.T) {
	s := stats.NewStats()
	y := &stats.Year{Commands: 12345, Failures: 383, Commits: 412, ActiveDays: 201, BestStreak: 14, LongestMs: 23*60000 + 10500,
		Cmds: map[string]int64{"git": 3210, "ls": 1998, "vim": 1998, "cd": 50, "make": 7, "rg": 6},
		Cwd:  map[string]int64{"repo": 7778, "home": 4567}}
	y.Hours[14] = 900
	y.Hours[15] = 900
	s.Years["2026"] = y
	for _, id := range []string{"first_steps", "warming_up", "keyboard_warrior", "shell_dweller", "escaped_vim", "night_owl", "rtfm"} {
		s.Unlocked[id] = t0
	}
	s.Unlocked["snapshot"] = time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC) // other year: excluded
	got, ok := Wrapped(s, 2026)
	want := strings.Join([]string{
		"╭────────────────────────────────────────────╮",
		"│ termachievements wrapped · 2026            │",
		"│                                            │",
		"│ 12,345 commands on 201 days                │",
		"│ 3.1% failed · best streak 14 days          │",
		"│ busiest hour 14:00-15:00                   │",
		"│ longest command 23m 10s                    │",
		"│ 412 git commits                            │",
		"│ mostly in: git repos (63%)                 │",
		"│                                            │",
		"│ top commands                               │",
		"│   1. git        3,210                      │",
		"│   2. ls         1,998                      │",
		"│   3. vim        1,998                      │",
		"│   4. cd         50                         │",
		"│   5. make       7                          │",
		"│                                            │",
		"│ 7 achievements unlocked                    │",
		"│   First Steps, Warming Up,                 │",
		"│   Keyboard Warrior, Shell Dweller,         │",
		"│   Escaped Vim, RTFM, Night Owl             │",
		"╰────────────────────────────────────────────╯",
	}, "\n") + "\n"
	if !ok || got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestWrappedEmptyAndHelpers(t *testing.T) {
	if _, ok := Wrapped(stats.NewStats(), 2026); ok {
		t.Fatal("empty year")
	}
	for ms, want := range map[int64]string{0: "0s", 59999: "59s", 60000: "1m 0s", 3600000: "1h 0m", 3*3600000 + 125000: "3h 2m"} {
		if got := dur(ms); got != want {
			t.Errorf("%d: %s", ms, got)
		}
	}
	if commas(1234567) != "1,234,567" || commas(999) != "999" {
		t.Fatal()
	}
}

func TestWrappedSingular(t *testing.T) {
	s := stats.NewStats()
	s.Years["2026"] = &stats.Year{Commands: 1, ActiveDays: 1, BestStreak: 1, Commits: 1, Cwd: map[string]int64{"tmp": 1}, Cmds: map[string]int64{}}
	s.Unlocked["first_steps"] = t0
	got, _ := Wrapped(s, 2026)
	for _, want := range []string{"│ 1 command on 1 day ", "│ 0.0% failed · best streak 1 day ", "│ 1 git commit ", "│ mostly in: temp dirs (100%) ", "│ busiest hour 00:00-01:00 ", "│   (none tracked yet) ", "│ 1 achievement unlocked "} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	delete(s.Unlocked, "first_steps")
	got, _ = Wrapped(s, 2026)
	if !strings.Contains(got, "│ 0 achievements unlocked ") {
		t.Errorf("missing zero line in\n%s", got)
	}
}

func TestWrappedTruncatesLongLine(t *testing.T) {
	s := stats.NewStats()
	s.Years["2026"] = &stats.Year{Commands: 1 << 62, ActiveDays: 1 << 62, Cwd: map[string]int64{}, Cmds: map[string]int64{}}
	got, _ := Wrapped(s, 2026)
	if !strings.Contains(got, "…") {
		t.Errorf("long line not cut:\n%s", got)
	}
}
