package cli

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/dpologdvinity/term-achievements/internal/store"
)

func rec(d Deps, exit string, line ...string) {
	Run(append([]string{"record", "--exit", exit, "--ms", "5", "--"}, line...), d)
}

func TestRecordFirstToast(t *testing.T) {
	d, out, errb := testDeps(t)
	rec(d, "0", "ls", "-la")
	if out.String() != "🏆 Achievement unlocked: First Steps — run your first command\n" || errb.Len() != 0 {
		t.Fatalf("%q %q", out, errb)
	}
	out.Reset()
	rec(d, "0", "ls")
	if out.Len() != 0 {
		t.Fatal(out)
	}
}

func TestRecordEscapedVim(t *testing.T) {
	d, out, _ := testDeps(t)
	for i := 0; i < 10; i++ {
		out.Reset()
		rec(d, "0", "vim", "x")
	}
	if out.String() != "🏆 Achievement unlocked: Escaped Vim — exit vim 10 times\n" {
		t.Fatalf("%q", out)
	}
}

func TestRecordStartedAndPrev(t *testing.T) {
	d, _, _ := testDeps(t)
	Run([]string{"record", "--exit", "0", "--started", strconv.FormatInt(t0.Unix()-700, 10), "--", "make"}, d)
	Run([]string{"record", "--exit", "0", "--prev", "apt update", "--", "sudo", "apt", "update"}, d)
	s, _, _ := store.Load(d.Getenv("TERM_ACHIEVEMENTS_HOME"), t0)
	if s.Counters["long_build"] != 1 || s.Counters["sudo_retry"] != 1 {
		t.Fatalf("%v", s.Counters)
	}
}

func TestRecordBadInputSilent(t *testing.T) {
	d, out, errb := testDeps(t)
	for _, args := range [][]string{{"record"}, {"record", "--exit", "x", "--", "ls"}, {"record", "--exit", "0", "--"}, {"record", "--bogus"}} {
		if Run(args, d) != 0 || out.Len()+errb.Len() != 0 {
			t.Fatalf("%v: %q %q", args, out, errb)
		}
	}
}

func TestRecordDisabled(t *testing.T) {
	d, out, _ := testDeps(t)
	d.Getenv = withEnv(d.Getenv, "TERM_ACHIEVEMENTS_DISABLE", "1")
	rec(d, "0", "ls")
	if out.Len() != 0 {
		t.Fatal(out)
	}
	if _, err := os.Stat(filepath.Join(d.Getenv("TERM_ACHIEVEMENTS_HOME"), "stats.json")); err == nil {
		t.Fatal("wrote state")
	}
}

func TestRecordPrivacy(t *testing.T) {
	d, _, _ := testDeps(t)
	secretDir := filepath.Join(t.TempDir(), "secretproject")
	os.MkdirAll(secretDir, 0o755)
	d.Getwd = func() (string, error) { return secretDir, nil }
	prev := []string{"record", "--exit", "0", "--prev", "export API_KEY=zzsecretzz", "--"}
	Run(append(slices.Clone(prev), "git", "commit", "-m", `"SECRET_TOKEN_abc123"`), d)
	Run(append(slices.Clone(prev), "mysql", "-phunter2pass"), d)
	Run(append(slices.Clone(prev), "hunter2pass"), d)
	dir := d.Getenv("TERM_ACHIEVEMENTS_HOME")
	files := 0
	filepath.WalkDir(dir, func(p string, e fs.DirEntry, _ error) error {
		if e.IsDir() {
			return nil
		}
		files++
		b, _ := os.ReadFile(p)
		for _, bad := range []string{"SECRET_TOKEN_abc123", "hunter2pass", "zzsecretzz", "secretproject", "-m", "mysql"} {
			if bytes.Contains(b, []byte(bad)) {
				t.Errorf("%s contains %q", p, bad)
			}
		}
		return nil
	})
	if files == 0 {
		t.Fatal("nothing was written, the test proves nothing")
	}
}

func TestRecordLockBusy(t *testing.T) {
	d, out, _ := testDeps(t)
	un, _ := store.Lock(d.Getenv("TERM_ACHIEVEMENTS_HOME"), false, time.Second)
	defer un()
	s := time.Now()
	rec(d, "0", "ls")
	if time.Since(s) > 50*time.Millisecond || out.Len() != 0 {
		t.Fatal("must drop the event fast and silently")
	}
}

func TestRecordCorruptSilent(t *testing.T) {
	d, out, errb := testDeps(t)
	dir := d.Getenv("TERM_ACHIEVEMENTS_HOME")
	os.WriteFile(filepath.Join(dir, "stats.json"), []byte("nope"), 0o600)
	rec(d, "0", "ls")
	if errb.Len() != 0 || out.String() != "🏆 Achievement unlocked: First Steps — run your first command\n" {
		t.Fatalf("%q %q", out, errb)
	}
}

func TestRecordBudget(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	d, _, _ := testDeps(t)
	d.Stdout = io.Discard
	for i := 0; i < 5000; i++ { // a busy history
		rec(d, strconv.Itoa(i%3), "git", "status")
	}
	var ts []time.Duration
	for i := 0; i < 20; i++ {
		s := time.Now()
		rec(d, "0", "ls")
		ts = append(ts, time.Since(s))
	}
	slices.Sort(ts)
	if ts[10] >= 10*time.Millisecond {
		t.Fatalf("median %v", ts[10])
	}
}

func BenchmarkRecord(b *testing.B) {
	home := b.TempDir()
	d := Deps{
		Now:      func() time.Time { return t0 },
		Getenv:   func(k string) string { return map[string]string{"TERM_ACHIEVEMENTS_HOME": home, "HOME": home}[k] },
		Getwd:    func() (string, error) { return home, nil },
		TempDirs: []string{"/tmp"},
		Stdout:   io.Discard,
		Stderr:   io.Discard,
	}
	for i := 0; i < b.N; i++ {
		rec(d, "0", "ls")
	}
}
