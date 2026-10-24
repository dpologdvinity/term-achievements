package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dpologdvinity/term-achievements/internal/stats"
)

// t0 is the fixed base time of the tests: a Tuesday at noon, UTC. Tests never read the real clock.
var t0 = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

// TestDir checks the state directory lookup order.
func TestDir(t *testing.T) {
	g := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if d, _ := Dir(g(map[string]string{"TERM_ACHIEVEMENTS_HOME": "/a", "HOME": "/h"})); d != "/a" {
		t.Fatal(d)
	}
	if d, _ := Dir(g(map[string]string{"XDG_STATE_HOME": "/s", "HOME": "/h"})); d != "/s/term-achievements" {
		t.Fatal(d)
	}
	if d, _ := Dir(g(map[string]string{"XDG_STATE_HOME": "s", "HOME": "/h"})); d != "/h/.local/state/term-achievements" {
		t.Fatal(d)
	}
	if _, err := Dir(g(nil)); err == nil {
		t.Fatal("no home must error")
	}
}

// TestRoundTrip checks that Save then Load returns the same data with the right file and directory modes.
func TestRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ta")
	s := stats.NewStats()
	s.Counters["vim"] = 3
	s.Unlocked["first_steps"] = t0
	if err := Save(dir, s); err != nil {
		t.Fatal(err)
	}
	got, moved, err := Load(dir, t0)
	if err != nil || moved != "" || got.Counters["vim"] != 3 || !got.Unlocked["first_steps"].Equal(t0) {
		t.Fatal(got, err)
	}
	if fi, _ := os.Stat(filepath.Join(dir, "stats.json")); fi.Mode().Perm() != 0o600 {
		t.Fatal(fi.Mode())
	}
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Fatal(fi.Mode())
	}
}

// TestLoadMissingAndNullMaps checks that a missing file and null JSON maps both load as usable empty stats.
func TestLoadMissingAndNullMaps(t *testing.T) {
	dir := t.TempDir()
	s, moved, err := Load(dir, t0)
	if err != nil || moved != "" || s.Version != 1 || s.Counters == nil {
		t.Fatal(s, err)
	}
	os.WriteFile(filepath.Join(dir, "stats.json"), []byte(`{"version":1,"counters":null,"unlocked":null,"years":null,"cmds":null}`), 0o600)
	s, _, err = Load(dir, t0)
	if err != nil || s.Counters == nil || s.Cmds == nil || s.Unlocked == nil || s.Years == nil {
		t.Fatal(s, err)
	}
}

// TestLoadCorrupt checks that an unparseable file is moved aside and empty stats are returned.
func TestLoadCorrupt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "stats.json"), []byte("{\"counters\":"), 0o600)
	s, moved, err := Load(dir, t0)
	if err != nil || len(s.Counters) != 0 || moved != filepath.Join(dir, fmt.Sprintf("stats.json.corrupt-%d", t0.Unix())) {
		t.Fatal(s, moved, err)
	}
	if _, err := os.Stat(moved); err != nil {
		t.Fatal("corrupt file not kept")
	}
}

// TestLoadNewer checks that a newer schema version is refused, even if its shape no longer parses.
func TestLoadNewer(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "stats.json"), []byte(`{"version":2}`), 0o600)
	if _, _, err := Load(dir, t0); !errors.Is(err, ErrNewer) {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "stats.json"), []byte(`{"version":3,"counters":[1]}`), 0o600)
	if _, _, err := Load(dir, t0); !errors.Is(err, ErrNewer) {
		t.Fatal("shape changes in newer versions must still read as newer:", err)
	}
}

// TestLockBusy checks that a held lock makes a second Lock fail quickly with ErrBusy.
func TestLockBusy(t *testing.T) {
	dir := t.TempDir()
	un, _ := Lock(dir, false, time.Second)
	defer un()
	s := time.Now()
	if _, err := Lock(dir, false, 5*time.Millisecond); !errors.Is(err, ErrBusy) || time.Since(s) > 50*time.Millisecond {
		t.Fatal(err)
	}
}

// TestLockSharedAndRelease checks that shared locks coexist and that unlock really releases the lock.
func TestLockSharedAndRelease(t *testing.T) {
	dir := t.TempDir()
	a, err1 := Lock(dir, true, time.Second)
	b, err2 := Lock(dir, true, time.Second)
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	a()
	b()
	un, err := Lock(dir, false, 5*time.Millisecond)
	if err != nil {
		t.Fatal("lock not released:", err)
	}
	un()
}
