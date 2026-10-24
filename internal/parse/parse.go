// Package parse extracts the few facts termachievements needs from a command line.
// The line is only ever held in memory.
package parse

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Cmd is the analyzed first command of a line.
type Cmd struct {
	Name      string // basename of the command word, "" if none
	Sudo      bool
	Args      []string // tokens after Name up to the first separator
	Sub       string   // git subcommand, "" if not git
	SubArgs   []string // tokens after Sub
	SudoRetry bool
}

var (
	envAssign   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	separators  = []string{"|", "||", "&&", ";", "&", "|&"}
	sudoArgOpts = []string{"-u", "-g", "-C", "-D", "-h", "-p", "-r", "-t", "-U", "-T"}
	wrappers    = []string{"time", "nohup", "command", "builtin", "exec", "nice", "env"}
	gitArgOpts  = []string{"-C", "-c", "--git-dir", "--work-tree", "--namespace"}
)

// Parse analyzes the first command of line. prev is the previous command line, used for sudo retries.
func Parse(line, prev string) Cmd {
	toks := strings.Fields(line)
	end := len(toks)
	for i, t := range toks {
		if slices.Contains(separators, t) {
			end = i
			break
		}
	}
	t := toks[:end]
	var c Cmd
	i, afterSudo := 0, 0

	skipEnv := func() {
		for i < len(t) && envAssign.MatchString(t[i]) {
			i++
		}
	}
	skipEnv()
	for i < len(t) {
		switch {
		case t[i] == "sudo" || t[i] == "doas":
			c.Sudo = true
			i++
			afterSudo = i
			for i < len(t) && strings.HasPrefix(t[i], "-") {
				if slices.Contains(sudoArgOpts, t[i]) {
					i++
				}
				i++
			}
		case slices.Contains(wrappers, t[i]):
			i++
			for i < len(t) && (strings.HasPrefix(t[i], "-") || envAssign.MatchString(t[i])) {
				i++
			}
		default:
			goto found
		}
		skipEnv()
	}
	return c
found:
	c.Name = filepath.Base(strings.TrimPrefix(t[i], "\\"))
	c.Args = t[i+1:]
	if c.Name == "git" {
		for j := 0; j < len(c.Args); j++ {
			a := c.Args[j]
			switch {
			case slices.Contains(gitArgOpts, a):
				j++
			case strings.HasPrefix(a, "-"):
			default:
				c.Sub, c.SubArgs = a, c.Args[j+1:]
				j = len(c.Args)
			}
		}
	}
	c.SudoRetry = c.Sudo && prev != "" &&
		strings.Join(t[afterSudo:], " ") == strings.Join(strings.Fields(prev), " ")
	return c
}
