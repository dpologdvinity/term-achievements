// Package store reads and writes stats.json under an advisory file lock.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dpologdvinity/term-achievements/internal/stats"
)

var (
	// ErrBusy means the lock stayed taken for the whole time budget.
	ErrBusy = errors.New("another term-achievements is busy; try again")
	// ErrNewer means stats.json has a newer schema than this build understands.
	ErrNewer = errors.New("stats were written by a newer term-achievements; please upgrade")
)

// Dir returns the state directory (SPEC §5.1).
func Dir(getenv func(string) string) (string, error) {
	if d := getenv("TERM_ACHIEVEMENTS_HOME"); d != "" {
		return d, nil
	}
	if x := getenv("XDG_STATE_HOME"); filepath.IsAbs(x) {
		return filepath.Join(x, "term-achievements"), nil
	}
	if h := getenv("HOME"); h != "" {
		return filepath.Join(h, ".local", "state", "term-achievements"), nil
	}
	return "", errors.New("cannot find a home directory")
}

// Lock takes LOCK_EX (or LOCK_SH when shared) on dir/stats.lock, retrying every 500µs for budget.
func Lock(dir string, shared bool, budget time.Duration) (unlock func(), err error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "stats.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	how := syscall.LOCK_EX
	if shared {
		how = syscall.LOCK_SH
	}
	deadline := time.Now().Add(budget)
	for {
		err := syscall.Flock(int(f.Fd()), how|syscall.LOCK_NB)
		if err == nil {
			// Closing the file would release the lock anyway; the explicit unlock is for clarity.
			return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EINTR {
			_ = f.Close()
			return nil, err
		}
		if !time.Now().Before(deadline) {
			_ = f.Close()
			return nil, ErrBusy
		}
		time.Sleep(500 * time.Microsecond)
	}
}

// Load reads dir/stats.json. A missing file gives empty stats. An unreadable one is moved aside
// (movedTo names it) and empty stats are returned. A newer version gives ErrNewer.
func Load(dir string, now time.Time) (s *stats.Stats, movedTo string, err error) {
	path := filepath.Join(dir, "stats.json")
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return stats.NewStats(), "", nil
	}
	if err != nil {
		return nil, "", err
	}
	s = stats.NewStats()
	if err := json.Unmarshal(b, s); err != nil {
		var v struct{ Version int }
		if json.Unmarshal(b, &v) == nil && v.Version > 1 {
			return nil, "", ErrNewer
		}
		movedTo = fmt.Sprintf("%s.corrupt-%d", path, now.Unix())
		if err := os.Rename(path, movedTo); err != nil {
			return nil, "", err
		}
		return stats.NewStats(), movedTo, nil
	}
	if s.Version > 1 {
		return nil, "", ErrNewer
	}
	s.Version = 1
	if s.Counters == nil {
		s.Counters = map[string]int64{}
	}
	if s.Cmds == nil {
		s.Cmds = map[string]int64{}
	}
	if s.Unlocked == nil {
		s.Unlocked = map[string]time.Time{}
	}
	if s.Years == nil {
		s.Years = map[string]*stats.Year{}
	}
	for k, y := range s.Years {
		if y == nil {
			delete(s.Years, k)
		}
	}
	return s, "", nil
}

// Save writes s atomically (temporary file, then rename) with mode 0600, creating dir with mode 0700.
func Save(dir string, s *stats.Stats) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, fmt.Sprintf("stats.json.tmp-%d", os.Getpid()))
	if err := os.WriteFile(tmp, b, 0o600); err != nil { // no fsync: it costs ~30ms on some disks, and record has a 10ms budget
		_ = os.Remove(tmp) // best effort: the original error is what matters
		return err
	}
	if err := os.Rename(tmp, filepath.Join(dir, "stats.json")); err != nil {
		_ = os.Remove(tmp) // best effort: the original error is what matters
		return err
	}
	return nil
}
