// Package render turns stats into the text term-achievements prints.
package render

import (
	"fmt"
	"strings"

	"github.com/dpologdvinity/term-achievements/internal/ach"
)

// maxToasts is the most lines a single record may print (SPEC §8.1).
const maxToasts = 3

// Toasts returns one line per new unlock (SPEC §8.1), each ending in a newline.
func Toasts(unlocked []ach.Achievement) string {
	show := unlocked
	if len(unlocked) > maxToasts {
		show = unlocked[:maxToasts-1]
	}
	var b strings.Builder
	for _, a := range show {
		fmt.Fprintf(&b, "🏆 Achievement unlocked: %s — %s\n", a.Title, a.Desc)
	}
	if len(show) < len(unlocked) {
		fmt.Fprintf(&b, "🏆 …and %d more (see term-achievements list)\n", len(unlocked)-len(show))
	}
	return b.String()
}
