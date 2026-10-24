package stats

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dpologdvinity/term-achievements/internal/parse"
)

var t0 = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

func ev(line string, exit int, ms int64, now time.Time, cwd string) Event {
	return Event{Cmd: parse.Parse(line, ""), Exit: exit, Ms: ms, Now: now, CwdType: cwd}
}

func TestCounters(t *testing.T) {
	sat8 := time.Date(2026, 3, 14, 8, 0, 0, 0, time.UTC) // Saturday 08:00
	night := time.Date(2026, 3, 10, 3, 59, 0, 0, time.UTC)
	early := time.Date(2026, 3, 10, 5, 0, 0, 0, time.UTC)
	cases := []struct {
		e    Event
		key  string
		want int64
	}{
		{ev("nvim x", 0, 1, t0, "other"), "vim", 1},
		{ev("emacsclient x", 1, 1, t0, "other"), "emacs", 1},
		{ev("rm -r -f build", 0, 1, t0, "other"), "rm_rf", 1},
		{ev("rm -Rf build", 1, 1, t0, "other"), "rm_rf", 0},
		{ev("rm --recursive --force x", 0, 1, t0, "other"), "rm_rf", 1},
		{ev("rm -- -rf", 0, 1, t0, "other"), "rm_rf", 0},
		{ev("gti status", 127, 1, t0, "other"), "gti", 1},
		{ev("gti status", 127, 1, t0, "other"), "typos", 1},
		{ev("gti status", 127, 1, t0, "other"), "not_found", 1},
		{ev("sl", 0, 1, t0, "other"), "toys", 1},
		{ev("man ls", 0, 1, t0, "other"), "man", 1},
		{ev("man nosuch", 16, 1, t0, "other"), "man", 0},
		{ev("git commit -m x", 0, 1, sat8, "repo"), "commits_before_9", 1},
		{ev("git commit -m x", 1, 1, sat8, "repo"), "commits", 0},
		{ev("git push -uf origin main", 0, 1, t0, "repo"), "force_push", 1},
		{ev("git push --force-with-lease=main origin", 0, 1, t0, "repo"), "force_push", 1},
		{ev("git push origin main", 0, 1, t0, "repo"), "force_push", 0},
		{ev("git rebase main", 1, 1, t0, "repo"), "rebase", 1},
		{ev("ls", 0, 1, night, "other"), "night", 1},
		{ev("ls", 0, 1, early, "other"), "early", 1},
		{ev("ls", 0, 1, sat8, "other"), "weekend", 1},
		{ev("ls", 0, 1, t0, "other"), "weekend", 0},
		{ev("./gradlew build", 0, 600000, t0, "other"), "long_build", 1},
		{ev("cargo build", 0, 599999, t0, "other"), "long_build", 0},
		{ev("sleep 4000", 0, 3600000, t0, "other"), "marathon", 1},
		{ev("chmod 777 x", 0, 1, t0, "other"), "chmod777", 1},
		{ev("kill -9 42", 0, 1, t0, "other"), "kill9", 1},
		{ev("sleep 1", 130, 1, t0, "other"), "interrupts", 1},
		{ev("sleep 1", 130, 1, t0, "other"), "failures", 0},
		{ev("false", 1, 1, t0, "other"), "failures", 1},
		{ev("ls", 0, 1, t0, "tmp"), "cwd_tmp", 1},
		{ev("ls", 0, 1, t0, "repo"), "cwd_repo", 1},
		{ev("sudo ls", 0, 1, t0, "other"), "sudo", 1},
		{ev("ls", 0, 1, t0, "other"), "commands", 1},
	}
	for _, c := range cases {
		s := NewStats()
		Apply(s, c.e)
		if got := s.Counters[c.key]; got != c.want {
			t.Errorf("%q exit %d: %s=%d want %d", strings.Join(append([]string{c.e.Cmd.Name}, c.e.Cmd.Args...), " "), c.e.Exit, c.key, got, c.want)
		}
	}
}

func TestTrackedOnly(t *testing.T) {
	s := NewStats()
	Apply(s, ev("hunter2pass", 127, 1, t0, "other"))
	Apply(s, ev("git status", 0, 1, t0, "repo"))
	if len(s.Cmds) != 1 || s.Cmds["git"] != 1 || s.Counters["distinct"] != 1 {
		t.Fatalf("%v", s.Cmds)
	}
	if y := s.Years["2026"]; y.Cmds["hunter2pass"] != 0 || y.Cmds["git"] != 1 || y.Cwd["repo"] != 1 || y.Cwd["other"] != 1 {
		t.Fatalf("%+v", y)
	}
}

func TestCwdType(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	tmp := filepath.Join(root, "tmp")
	repo := filepath.Join(home, "src", "p")
	os.MkdirAll(filepath.Join(repo, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repo, "deep"), 0o755)
	os.MkdirAll(filepath.Join(tmp, "x"), 0o755)
	cases := map[string]string{home: "home", tmp: "tmp", filepath.Join(tmp, "x"): "tmp", filepath.Join(repo, "deep"): "repo", filepath.Join(home, "src"): "other"}
	if CwdType(root, "", nil) == "repo" { // a .git above the temp dir (e.g. /tmp/.git) makes "other" unreachable here
		delete(cases, filepath.Join(home, "src"))
	}
	for cwd, want := range cases {
		if got := CwdType(cwd, home, []string{tmp}); got != want {
			t.Errorf("%s: %s want %s", cwd, got, want)
		}
	}
}
