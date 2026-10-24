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

// rec records a command with the given exit status through the real Run entry point.
func rec(d Deps, exit string, line ...string) {
	Run(append([]string{"record", "--exit", exit, "--ms", "5", "--"}, line...), d)
}

// TestRecordFirstToast checks that the first command prints the First Steps toast, and the second prints nothing.
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

// TestRecordEscapedVim checks that the tenth vim run prints the Escaped Vim toast.
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

// TestRecordStartedAndPrev checks the --started duration and --prev sudo-retry inputs.
func TestRecordStartedAndPrev(t *testing.T) {
	d, _, _ := testDeps(t)
	Run([]string{"record", "--exit", "0", "--started", strconv.FormatInt(t0.Unix()-700, 10), "--", "make"}, d)
	Run([]string{"record", "--exit", "0", "--prev", "apt update", "--", "sudo", "apt", "update"}, d)
	s, _, _ := store.Load(d.Getenv("TERM_ACHIEVEMENTS_HOME"), t0)
	if s.Counters["long_build"] != 1 || s.Counters["sudo_retry"] != 1 {
		t.Fatalf("%v", s.Counters)
	}
}

// TestRecordBadInputSilent checks that bad or missing flags print nothing and exit 0.
func TestRecordBadInputSilent(t *testing.T) {
	d, out, errb := testDeps(t)
	for _, args := range [][]string{{"record"}, {"record", "--exit", "x", "--", "ls"}, {"record", "--exit", "0", "--"}, {"record", "--bogus"}} {
		if Run(args, d) != 0 || out.Len()+errb.Len() != 0 {
			t.Fatalf("%v: %q %q", args, out, errb)
		}
	}
}

// TestRecordDisabled checks that TERM_ACHIEVEMENTS_DISABLE stops all recording and writing.
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

// TestRecordPrivacy checks that secrets in the command, previous command and directory name never reach any file.
func TestRecordPrivacy(t *testing.T) {
	d, _, _ := testDeps(t)
	secretDir := filepath.Join(t.TempDir(), "secretproject")
	if err := os.MkdirAll(secretDir, 0o755); err != nil {
		t.Fatal(err)
	}
	d.Getwd = func() (string, error) { return secretDir, nil }
	prev := []string{"record", "--exit", "0", "--prev", "export API_KEY=zzsecretzz", "--"}
	Run(append(slices.Clone(prev), "git", "commit", "-m", `"SECRET_TOKEN_abc123"`), d)
	Run(append(slices.Clone(prev), "mysql", "-phunter2pass"), d)
	Run(append(slices.Clone(prev), "hunter2pass"), d)
	dir := d.Getenv("TERM_ACHIEVEMENTS_HOME")
	files := 0
	err := filepath.WalkDir(dir, func(p string, e fs.DirEntry, _ error) error {
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
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Fatal("nothing was written, the test proves nothing")
	}
}

// TestRecordLockBusy checks that a held lock drops the event quickly and silently.
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

// TestRecordCorruptSilent checks that record starts fresh on a corrupt file without printing a warning.
func TestRecordCorruptSilent(t *testing.T) {
	d, out, errb := testDeps(t)
	dir := d.Getenv("TERM_ACHIEVEMENTS_HOME")
	writeStats(t, dir, "nope")
	rec(d, "0", "ls")
	if errb.Len() != 0 || out.String() != "🏆 Achievement unlocked: First Steps — run your first command\n" {
		t.Fatalf("%q %q", out, errb)
	}
}

// TestRecordBudget checks that the median record run stays under the 10 ms budget (skipped with -short).
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

// BenchmarkRecord measures one record call; run with `go test -bench Record -run ^$ ./internal/cli/`.
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
