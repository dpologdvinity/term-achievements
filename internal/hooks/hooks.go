// Package hooks serves the embedded shell hook scripts.
package hooks

import "embed"

//go:embed termachievements.zsh termachievements.bash termachievements.fish
var files embed.FS

// Script returns the hook for shell ("zsh", "bash" or "fish").
func Script(shell string) (string, bool) {
	switch shell {
	case "zsh", "bash", "fish":
		b, err := files.ReadFile("termachievements." + shell)
		return string(b), err == nil
	}
	return "", false
}
