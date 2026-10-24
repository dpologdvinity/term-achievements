package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/dpologdvinity/term-achievements/internal/ach"
	"github.com/dpologdvinity/term-achievements/internal/stats"
)

// List returns the header and one line per achievement (SPEC §8.2).
func List(s *stats.Stats, loc *time.Location) string {
	var rows strings.Builder
	done := 0
	for _, a := range ach.All {
		mark, right := "·", ""
		if at, ok := s.Unlocked[a.ID]; ok {
			done++
			mark, right = "✓", at.In(loc).Format("2006-01-02")
		} else {
			cur := min(s.Counters[a.Counter], a.Goal)
			k := int(cur * 10 / a.Goal)
			right = fmt.Sprintf("[%s%s] %d/%d", strings.Repeat("#", k), strings.Repeat("-", 10-k), cur, a.Goal)
		}
		rows.WriteString(strings.TrimRight(fmt.Sprintf("%s %-26s %-44s %s", mark, a.Title, a.Desc, right), " ") + "\n")
	}
	return fmt.Sprintf("term-achievements · %d/%d unlocked\n\n%s", done, len(ach.All), rows.String())
}
