// Package hooks serves the embedded shell hook scripts.
package hooks

import "embed"

// files holds the hook scripts, compiled into the binary so `init` needs no data files.
//
//go:embed term-achievements.zsh term-achievements.bash term-achievements.fish
var files embed.FS

// Script returns the hook for shell ("zsh", "bash" or "fish").
func Script(shell string) (string, bool) {
	switch shell {
	case "zsh", "bash", "fish":
		b, err := files.ReadFile("term-achievements." + shell)
		return string(b), err == nil
	}
	return "", false
}
