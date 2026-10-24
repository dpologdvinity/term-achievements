package cli

import (
	"os"
	"path/filepath"

	"bytes"
	"github.com/dpologdvinity/termachievements/internal/hooks"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

func withEnv(g func(string) string, k, v string) func(string) string {
	return func(key string) string {
		if key == k {
			return v
		}
		return g(key)
	}
}

func testDeps(t *testing.T) (Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	env := map[string]string{"TERMACHIEVEMENTS_HOME": t.TempDir(), "HOME": t.TempDir()}
	cwd := t.TempDir()
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	return Deps{
		Now:      func() time.Time { return t0 },
		Getenv:   func(k string) string { return env[k] },
		Getwd:    func() (string, error) { return cwd, nil },
		TempDirs: []string{t.TempDir()},
		Stdout:   out,
		Stderr:   errb,
	}, out, errb
}

func TestVersionHelpUnknown(t *testing.T) {
	d, out, errb := testDeps(t)
	d.Version = "0.1.0"
	if Run([]string{"--version"}, d) != 0 || out.String() != "termachievements 0.1.0\n" {
		t.Fatal(out)
	}
	out.Reset()
	if Run([]string{"help"}, d) != 0 || !strings.HasPrefix(out.String(), "termachievements - achievements for your shell\n") {
		t.Fatal(out)
	}
	if Run([]string{"nope"}, d) != 2 || errb.String() != "termachievements: unknown command \"nope\" (see termachievements --help)\n" {
		t.Fatal(errb)
	}
}

func TestListCommand(t *testing.T) {
	d, out, _ := testDeps(t)
	if Run(nil, d) != 0 || !strings.HasPrefix(out.String(), "termachievements · 0/40 unlocked\n\n") {
		t.Fatal(out)
	}
	rec(d, "0", "ls")
	out.Reset()
	if Run([]string{"list"}, d) != 0 || !strings.Contains(out.String(), "✓ First Steps") {
		t.Fatal(out)
	}
}

func TestListCorruptWarns(t *testing.T) {
	d, _, errb := testDeps(t)
	os.WriteFile(filepath.Join(d.Getenv("TERMACHIEVEMENTS_HOME"), "stats.json"), []byte("nope"), 0o600)
	if Run([]string{"list"}, d) != 0 || !strings.HasPrefix(errb.String(), "termachievements: stats file was unreadable; moved it to ") {
		t.Fatal(errb)
	}
	errb.Reset()
	if Run([]string{"list"}, d) != 0 || errb.Len() != 0 {
		t.Fatal("warns only once:", errb)
	}
}

func TestListNewerAndBusy(t *testing.T) {
	d, _, errb := testDeps(t)
	dir := d.Getenv("TERMACHIEVEMENTS_HOME")
	os.WriteFile(filepath.Join(dir, "stats.json"), []byte(`{"version":2}`), 0o600)
	if Run([]string{"list"}, d) != 1 || errb.String() != "termachievements: stats were written by a newer termachievements; please upgrade\n" {
		t.Fatal(errb)
	}
}

func TestWrappedCommand(t *testing.T) {
	d, out, errb := testDeps(t)
	if Run([]string{"wrapped"}, d) != 0 || out.String() != "no commands recorded in 2026 yet\n" {
		t.Fatal(out)
	}
	rec(d, "0", "git", "status")
	out.Reset()
	if Run([]string{"wrapped", "--year", "2026"}, d) != 0 || !strings.Contains(out.String(), "│ 1 command on 1 day") {
		t.Fatal(out)
	}
	if Run([]string{"wrapped", "--year", "twenty"}, d) != 2 || errb.String() != "termachievements: --year must be a year like 2026\n" {
		t.Fatal(errb)
	}
}

func TestInitCommand(t *testing.T) {
	d, out, errb := testDeps(t)
	want, _ := hooks.Script("bash")
	if Run([]string{"init", "bash"}, d) != 0 || out.String() != want {
		t.Fatal(out)
	}
	if Run([]string{"init"}, d) != 2 || errb.String() != "termachievements: init needs a shell: zsh, bash or fish\n" {
		t.Fatal(errb)
	}
}
