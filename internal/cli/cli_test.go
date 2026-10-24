package cli

import (
	"bytes"
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
