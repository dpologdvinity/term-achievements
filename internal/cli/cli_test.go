package cli

import (
	"os"
	"path/filepath"

	"bytes"
	"github.com/dpologdvinity/term-achievements/internal/hooks"
	"strings"
	"testing"
	"time"
)

// t0 is the fixed base time of the tests: a Tuesday at noon, UTC. Tests never read the real clock.
var t0 = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

// writeStats puts content into dir/stats.json.
func writeStats(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "stats.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// withEnv wraps getenv so that key k returns v.
func withEnv(g func(string) string, k, v string) func(string) string {
	return func(key string) string {
		if key == k {
			return v
		}
		return g(key)
	}
}

// testDeps returns Deps wired to temporary directories, a fixed clock and captured output.
func testDeps(t *testing.T) (Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	env := map[string]string{"TERM_ACHIEVEMENTS_HOME": t.TempDir(), "HOME": t.TempDir()}
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

// TestVersionHelpUnknown checks --version, help and the unknown-command error.
func TestVersionHelpUnknown(t *testing.T) {
	d, out, errb := testDeps(t)
	d.Version = "0.1.0"
	if Run([]string{"--version"}, d) != 0 || out.String() != "term-achievements 0.1.0\n" {
		t.Fatal(out)
	}
	out.Reset()
	if Run([]string{"help"}, d) != 0 || !strings.HasPrefix(out.String(), "term-achievements - achievements for your shell\n") {
		t.Fatal(out)
	}
	if Run([]string{"nope"}, d) != 2 || errb.String() != "term-achievements: unknown command \"nope\" (see term-achievements --help)\n" {
		t.Fatal(errb)
	}
}

// TestListCommand checks that list works on empty and non-empty stats, with no arguments or "list".
func TestListCommand(t *testing.T) {
	d, out, _ := testDeps(t)
	if Run(nil, d) != 0 || !strings.HasPrefix(out.String(), "term-achievements · 0/40 unlocked\n\n") {
		t.Fatal(out)
	}
	rec(d, "0", "ls")
	out.Reset()
	if Run([]string{"list"}, d) != 0 || !strings.Contains(out.String(), "✓ First Steps") {
		t.Fatal(out)
	}
}

// TestListCorruptWarns checks that list warns once about a corrupt stats file.
func TestListCorruptWarns(t *testing.T) {
	d, _, errb := testDeps(t)
	writeStats(t, d.Getenv("TERM_ACHIEVEMENTS_HOME"), "nope")
	if Run([]string{"list"}, d) != 0 || !strings.HasPrefix(errb.String(), "term-achievements: stats file was unreadable; moved it to ") {
		t.Fatal(errb)
	}
	errb.Reset()
	if Run([]string{"list"}, d) != 0 || errb.Len() != 0 {
		t.Fatal("warns only once:", errb)
	}
}

// TestListNewerAndBusy checks that list refuses stats written by a newer version.
func TestListNewerAndBusy(t *testing.T) {
	d, _, errb := testDeps(t)
	dir := d.Getenv("TERM_ACHIEVEMENTS_HOME")
	writeStats(t, dir, `{"version":2}`)
	if Run([]string{"list"}, d) != 1 || errb.String() != "term-achievements: stats were written by a newer term-achievements; please upgrade\n" {
		t.Fatal(errb)
	}
}

// TestWrappedCommand checks the wrapped command, its empty-year message and --year validation.
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
	if Run([]string{"wrapped", "--year", "twenty"}, d) != 2 || errb.String() != "term-achievements: --year must be a year like 2026\n" {
		t.Fatal(errb)
	}
}

// TestInitCommand checks that init prints the embedded hook and rejects a missing shell.
func TestInitCommand(t *testing.T) {
	d, out, errb := testDeps(t)
	want, _ := hooks.Script("bash")
	if Run([]string{"init", "bash"}, d) != 0 || out.String() != want {
		t.Fatal(out)
	}
	if Run([]string{"init"}, d) != 2 || errb.String() != "term-achievements: init needs a shell: zsh, bash or fish\n" {
		t.Fatal(errb)
	}
}
