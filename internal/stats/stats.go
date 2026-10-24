// Package stats holds the persisted counters and applies finished commands to them.
// Only fixed counter keys and tracked command names are ever stored.
package stats

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dpologdvinity/term-achievements/internal/parse"
)

// Stats is the content of stats.json: everything term-achievements remembers.
// It holds counters and aggregates only, never command lines (SPEC §5.4).
type Stats struct {
	Version    int                  `json:"version"`
	Counters   map[string]int64     `json:"counters"`
	Cmds       map[string]int64     `json:"cmds"`
	LastDay    string               `json:"last_day"`
	Streak     int64                `json:"streak"`
	FailStreak int64                `json:"fail_streak"`
	Unlocked   map[string]time.Time `json:"unlocked"`
	Years      map[string]*Year     `json:"years"`
}

// Year holds the aggregates behind `wrapped`.
type Year struct {
	Commands   int64            `json:"commands"`
	Failures   int64            `json:"failures"`
	Commits    int64            `json:"commits"`
	ActiveDays int64            `json:"active_days"`
	BestStreak int64            `json:"best_streak"`
	LongestMs  int64            `json:"longest_ms"`
	Hours      [24]int64        `json:"hours"`
	Cmds       map[string]int64 `json:"cmds"`
	Cwd        map[string]int64 `json:"cwd"`
}

// Event is one finished command.
type Event struct {
	Cmd     parse.Cmd
	Exit    int
	Ms      int64
	Now     time.Time // local time at record
	CwdType string    // "home" | "tmp" | "repo" | "other"
}

// NewStats returns empty stats with every map allocated.
func NewStats() *Stats {
	return &Stats{
		Version:  1,
		Counters: map[string]int64{},
		Cmds:     map[string]int64{},
		Unlocked: map[string]time.Time{},
		Years:    map[string]*Year{},
	}
}

// set builds a lookup set from a whitespace-separated list of names.
func set(names string) map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Fields(names) {
		m[n] = true
	}
	return m
}

var (
	// Tracked is the only set of command names that may be stored by name.
	Tracked = set(`ls ll la exa eza cd pwd cat less more bat head tail grep rg ag find fd fzf
git gh vi vim nvim emacs emacsclient nano code
make cmake ninja cargo rustc go npm npx pnpm yarn bun deno node tsc
python python3 pip pip3 uv ruby gem bundle java javac gradle gradlew mvn
bazel dotnet swift xcodebuild
docker podman kubectl helm terraform ansible ssh scp rsync curl wget
man tldr sudo top htop btop tmux screen clear history echo
mkdir rm mv cp touch chmod chown ln kill pkill killall ps df du tar zip unzip
brew apt dnf pacman jq sqlite3 psql
sl cowsay fortune cmatrix figlet lolcat toilet asciiquarium
gti got gut gerp grpe claer clera mkae amke pyhton pytohn sudp suod vmi nvmi exti eixt`)
	// Typos are common mistypes of everyday commands.
	Typos = set("gti got gut gerp grpe claer clera mkae amke pyhton pytohn sudp suod vmi nvmi exti eixt")
	// Toys are fun programs such as cowsay and sl.
	Toys = set("sl cowsay fortune cmatrix figlet lolcat toilet asciiquarium")
	// BuildTools are build commands; a long successful run counts as a long build.
	BuildTools = set("make cmake ninja cargo go npm pnpm yarn bun gradle gradlew mvn bazel dotnet swift xcodebuild tsc")
)

// CounterKeys is every key that may appear in Stats.Counters.
var CounterKeys = strings.Fields(`commands failures not_found interrupts vim emacs nano rm_rf gti typos sl toys
sudo sudo_retry man commits commits_before_9 force_push rebase bisect reflog stash night early weekend
long_build marathon chmod777 kill9 clear cwd_tmp cwd_repo comebacks best_streak distinct`)

// dayFmt is the layout of Stats.LastDay: a local calendar day.
const dayFmt = "2006-01-02"

// Apply adds one finished command to s (SPEC §6.3). It never stores the command line:
// only fixed counter keys and names from Tracked are written.
//
// The order matters: counters are bumped first (comebacks reads FailStreak before it
// resets), then FailStreak, tracked names, the daily streak and the per-year aggregates.
func Apply(s *Stats, e Event) {
	c, n := e.Cmd, e.Cmd.Name
	fail := e.Exit != 0 && e.Exit != 130 && e.Exit != 148
	gitOK := func(sub string) bool { return n == "git" && c.Sub == sub && e.Exit == 0 }
	h := e.Now.Hour()
	// inc adds 1 to the counter key when cond holds.
	inc := func(cond bool, key string) {
		if cond {
			s.Counters[key]++
		}
	}

	inc(true, "commands")
	inc(fail, "failures")
	inc(e.Exit == 127, "not_found")
	inc(e.Exit == 130, "interrupts")
	inc(n == "vi" || n == "vim" || n == "nvim", "vim")
	inc(n == "emacs" || n == "emacsclient", "emacs")
	inc(n == "nano", "nano")
	inc(n == "rm" && e.Exit == 0 && rmRF(c.Args), "rm_rf")
	inc(n == "gti", "gti")
	inc(Typos[n], "typos")
	inc(n == "sl", "sl")
	inc(Toys[n], "toys")
	inc(c.Sudo, "sudo")
	inc(c.SudoRetry, "sudo_retry")
	inc(n == "man" && e.Exit == 0, "man")
	inc(gitOK("commit"), "commits")
	inc(gitOK("commit") && h < 9, "commits_before_9")
	inc(gitOK("push") && forceFlag(c.SubArgs), "force_push")
	inc(n == "git" && c.Sub == "rebase", "rebase")
	inc(n == "git" && c.Sub == "bisect", "bisect")
	inc(n == "git" && c.Sub == "reflog", "reflog")
	inc(n == "git" && c.Sub == "stash", "stash")
	inc(h <= 3, "night")
	inc(h == 5 || h == 6, "early")
	inc(e.Now.Weekday() == time.Saturday || e.Now.Weekday() == time.Sunday, "weekend")
	inc(BuildTools[n] && e.Ms >= 600000 && e.Exit == 0, "long_build")
	inc(e.Ms >= 3600000, "marathon")
	inc(n == "chmod" && e.Exit == 0 && slices.ContainsFunc(c.Args, func(a string) bool { return a == "777" || a == "0777" || a == "a+rwx" }), "chmod777")
	inc((n == "kill" || n == "pkill" || n == "killall") && slices.ContainsFunc(c.Args, func(a string) bool { return a == "-9" || a == "-KILL" || a == "-SIGKILL" }), "kill9")
	inc(n == "clear", "clear")
	inc(e.CwdType == "tmp", "cwd_tmp")
	inc(e.CwdType == "repo", "cwd_repo")
	inc(e.Exit == 0 && s.FailStreak >= 10, "comebacks")

	// Consecutive failures: a failure adds 1, a success resets, Ctrl-C/Ctrl-Z leave it alone.
	switch {
	case fail:
		s.FailStreak++
	case e.Exit == 0:
		s.FailStreak = 0
	}

	if Tracked[n] {
		s.Cmds[n]++
	}
	s.Counters["distinct"] = int64(len(s.Cmds))

	// Daily streak: +1 on the day after LastDay, back to 1 after a gap, ignored when the
	// clock goes backwards. Only the first command of each day does any work here.
	y := s.year(e.Now.Format("2006"))
	if day := e.Now.Format(dayFmt); day >= s.LastDay { // an earlier day means the clock went back
		if day != s.LastDay {
			if s.LastDay == e.Now.AddDate(0, 0, -1).Format(dayFmt) {
				s.Streak++
			} else {
				s.Streak = 1
			}
			s.LastDay = day
			y.ActiveDays++
			s.Counters["best_streak"] = max(s.Counters["best_streak"], s.Streak)
			y.BestStreak = max(y.BestStreak, s.Streak)
		}
	}

	// Per-year aggregates for `wrapped`.
	y.Commands++
	if fail {
		y.Failures++
	}
	if gitOK("commit") {
		y.Commits++
	}
	y.LongestMs = max(y.LongestMs, e.Ms)
	y.Hours[h]++
	if Tracked[n] {
		y.Cmds[n]++
	}
	y.Cwd[e.CwdType]++
}

// year returns the aggregates for key, creating them (and repairing nil maps from old files) when missing.
func (s *Stats) year(key string) *Year {
	y := s.Years[key]
	if y == nil {
		y = &Year{}
		s.Years[key] = y
	}
	if y.Cmds == nil {
		y.Cmds = map[string]int64{}
	}
	if y.Cwd == nil {
		y.Cwd = map[string]int64{}
	}
	return y
}

// rmRF reports whether args carry both a recursive and a force flag before any "--".
func rmRF(args []string) bool {
	var r, f bool
	for _, a := range args {
		switch {
		case a == "--":
			return r && f
		case a == "--recursive":
			r = true
		case a == "--force":
			f = true
		case strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--"):
			r = r || strings.ContainsAny(a, "rR")
			f = f || strings.Contains(a, "f")
		}
	}
	return r && f
}

// forceFlag reports whether git push arguments ask for a forced push: --force,
// --force-with-lease[=...], or a short option group containing f (such as -uf).
func forceFlag(args []string) bool {
	return slices.ContainsFunc(args, func(a string) bool {
		return a == "--force" || a == "--force-with-lease" || strings.HasPrefix(a, "--force-with-lease=") ||
			(strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "f"))
	})
}

// CwdType classifies cwd as "home", "tmp", "repo" or "other". Nothing about cwd is stored.
func CwdType(cwd, home string, tmps []string) string {
	if cwd == "" {
		return "other"
	}
	cwd = filepath.Clean(cwd)
	if home != "" && cwd == filepath.Clean(home) {
		return "home"
	}
	for _, t := range tmps {
		t = filepath.Clean(t)
		if cwd == t || strings.HasPrefix(cwd, t+string(filepath.Separator)) {
			return "tmp"
		}
	}
	dir := cwd
	for range 65 { // cwd plus up to 64 ancestors
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return "repo"
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "other"
}
