package render

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dpologdvinity/term-achievements/internal/ach"
	"github.com/dpologdvinity/term-achievements/internal/stats"
)

const cardWidth = 42

var places = []struct{ key, name string }{
	{"repo", "git repos"}, {"home", "your home dir"}, {"tmp", "temp dirs"}, {"other", "other places"},
}

// Wrapped returns the recap card for year (SPEC §8.3), or false when the year has no commands.
func Wrapped(s *stats.Stats, year int) (string, bool) {
	y := s.Years[strconv.Itoa(year)]
	if y == nil || y.Commands == 0 {
		return "", false
	}
	busiest := 0
	for h, n := range y.Hours {
		if n > y.Hours[busiest] {
			busiest = h
		}
	}
	place, best := places[0], int64(-1)
	for _, p := range places {
		if y.Cwd[p.key] > best {
			place, best = p, y.Cwd[p.key]
		}
	}

	lines := []string{
		fmt.Sprintf("term-achievements wrapped · %d", year),
		"",
		fmt.Sprintf("%s on %s", plural(y.Commands, "command"), plural(y.ActiveDays, "day")),
		fmt.Sprintf("%.1f%% failed · best streak %s", float64(y.Failures)*100/float64(y.Commands), plural(y.BestStreak, "day")),
		fmt.Sprintf("busiest hour %02d:00-%02d:00", busiest, (busiest+1)%24),
		"longest command " + dur(y.LongestMs),
		plural(y.Commits, "git commit"),
		fmt.Sprintf("mostly in: %s (%d%%)", place.name, int64(math.Round(float64(best)*100/float64(y.Commands)))),
		"",
		"top commands",
	}
	lines = append(lines, topCommands(y.Cmds)...)

	var titles []string
	for _, a := range ach.All {
		if at, ok := s.Unlocked[a.ID]; ok && at.Year() == year {
			titles = append(titles, a.Title)
		}
	}
	lines = append(lines, "", plural(int64(len(titles)), "achievement")+" unlocked")
	lines = append(lines, wrapTitles(titles)...)

	var b strings.Builder
	b.WriteString("╭" + strings.Repeat("─", cardWidth+2) + "╮\n")
	for _, l := range lines {
		if utf8.RuneCountInString(l) > cardWidth {
			l = string([]rune(l)[:cardWidth-1]) + "…"
		}
		b.WriteString("│ " + l + strings.Repeat(" ", cardWidth-utf8.RuneCountInString(l)) + " │\n")
	}
	b.WriteString("╰" + strings.Repeat("─", cardWidth+2) + "╯\n")
	return b.String(), true
}

func topCommands(cmds map[string]int64) []string {
	names := make([]string, 0, len(cmds))
	for n := range cmds {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if cmds[names[i]] != cmds[names[j]] {
			return cmds[names[i]] > cmds[names[j]]
		}
		return names[i] < names[j]
	})
	if len(names) == 0 {
		return []string{"  (none tracked yet)"}
	}
	var out []string
	for i, n := range names[:min(5, len(names))] {
		out = append(out, fmt.Sprintf("  %d. %-10s %s", i+1, n, commas(cmds[n])))
	}
	return out
}

// wrapTitles joins titles with commas and wraps them greedily to the card width.
func wrapTitles(titles []string) []string {
	var out []string
	for i, t := range titles {
		if i < len(titles)-1 {
			t += ","
		}
		if n := len(out); n > 0 && utf8.RuneCountInString(out[n-1])+1+utf8.RuneCountInString(t) <= cardWidth {
			out[n-1] += " " + t
		} else {
			out = append(out, "  "+t)
		}
	}
	return out
}

func plural(n int64, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return commas(n) + " " + word + "s"
}

func commas(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		s = "-" + s
	}
	return s
}

func dur(ms int64) string {
	sec := ms / 1000
	switch {
	case sec >= 3600:
		return fmt.Sprintf("%dh %dm", sec/3600, sec%3600/60)
	case sec >= 60:
		return fmt.Sprintf("%dm %ds", sec/60, sec%60)
	}
	return fmt.Sprintf("%ds", sec)
}
