// Package parse extracts the few facts term-achievements needs from a command line.
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
	// envAssign matches a leading environment assignment such as FOO=1.
	envAssign = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	// separators end the first command of a pipeline or command list.
	separators = []string{"|", "||", "&&", ";", "&", "|&"}
	// sudoArgOpts are sudo options that take a value, which must be skipped too.
	sudoArgOpts = []string{"-u", "-g", "-C", "-D", "-h", "-p", "-r", "-t", "-U", "-T"}
	// wrapperOpts are wrapper options that take a value (nice -n N, env -u NAME, env -C DIR).
	wrapperOpts = []string{"-n", "-u", "-C"} // nice -n N, env -u NAME, env -C DIR
	// wrappers are commands that only launch another command; the real name comes after them.
	wrappers = []string{"time", "nohup", "command", "builtin", "exec", "nice", "env"}
	// gitArgOpts are global git options that take a value, before the subcommand.
	gitArgOpts = []string{"-C", "-c", "--git-dir", "--work-tree", "--namespace"}
)

// Parse analyzes the first command of line (SPEC §6.1). prev is the previous command line;
// it is only compared with the current one to detect "sudo !!" and is never kept.
// Quoting is not interpreted: the line is split on whitespace.
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

	// skipEnv advances i past any leading VAR=value tokens.
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
				if slices.Contains(wrapperOpts, t[i]) {
					i++
				}
				i++
			}
		default:
			goto found
		}
		skipEnv()
	}
	return c
found:
	// Name is the basename, so "/usr/bin/make" and "\make" both count as "make".
	c.Name = filepath.Base(strings.TrimPrefix(t[i], "\\"))
	c.Args = t[i+1:]
	// Find the git subcommand, skipping global options such as -C <dir> and --no-pager.
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
	// "sudo !!" expands to sudo plus the previous line, so compare what follows sudo with prev.
	c.SudoRetry = c.Sudo && prev != "" &&
		strings.Join(t[afterSudo:], " ") == strings.Join(strings.Fields(prev), " ")
	return c
}
