package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeHook(t *testing.T, shell string) string {
	t.Helper()
	s, ok := Script(shell)
	if !ok {
		t.Fatalf("no script for %s", shell)
	}
	p := filepath.Join(t.TempDir(), "hook."+shell)
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// fakeBin installs a termachievements that appends its argv, '|'-separated, to a log.
func fakeBin(t *testing.T) (binDir, logPath string) {
	t.Helper()
	binDir, logPath = t.TempDir(), filepath.Join(t.TempDir(), "log")
	script := "#!/bin/sh\nprintf '%s|' \"$@\" >> " + logPath + "\necho >> " + logPath + "\n"
	if err := os.WriteFile(filepath.Join(binDir, "termachievements"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binDir, logPath
}

func TestScript(t *testing.T) {
	for _, sh := range []string{"zsh", "bash", "fish"} {
		if s, ok := Script(sh); !ok || !strings.Contains(s, "termachievements hook for "+sh) {
			t.Errorf("%s: %v", sh, ok)
		}
	}
	if s, ok := Script("tcsh"); ok || s != "" {
		t.Error("tcsh must be unknown")
	}
}

func TestBashHookInteractive(t *testing.T) {
	hookFile := writeHook(t, "bash")
	bin, log := fakeBin(t)
	cmd := exec.Command("bash", "--norc", "-i")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir(), "HISTFILE=/dev/null", "TERM=dumb"}
	cmd.Stdin = strings.NewReader("source " + hookFile + "\nfalse\ntrue  x\n\n")
	cmd.Run()
	b, _ := os.ReadFile(log)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("%q", b)
	}
	if !regexp.MustCompile(`^record\|--exit\|1\|--started\|\d+\|--prev\|\|--\|false\|$`).MatchString(lines[0]) {
		t.Fatal(lines[0])
	}
	if !regexp.MustCompile(`^record\|--exit\|0\|--started\|\d+\|--prev\|false\|--\|true  x\|$`).MatchString(lines[1]) {
		t.Fatal(lines[1])
	}
}

func TestZshHook(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not installed")
	}
	hookFile := writeHook(t, "zsh")
	bin, log := fakeBin(t)
	out, err := exec.Command("zsh", "-f", "-c", `PATH=`+bin+`:$PATH; source `+hookFile+`; source `+hookFile+`; _termach_preexec "make all"; (exit 3); _termach_precmd; echo "rc=$?"; _termach_precmd; echo "rc=$?"`).CombinedOutput()
	if string(out) != "rc=3\nrc=0\n" || err != nil {
		t.Fatalf("%q %v", out, err)
	}
	b, _ := os.ReadFile(log)
	if !regexp.MustCompile(`^record\|--exit\|3\|--ms\|\d+\|--prev\|\|--\|make all\|\n$`).Match(b) { // second precmd records nothing
		t.Fatalf("%q", b)
	}
}

func TestFishHook(t *testing.T) {
	if _, err := exec.LookPath("fish"); err != nil {
		t.Skip("fish not installed")
	}
	hookFile := writeHook(t, "fish")
	bin, log := fakeBin(t)
	out, _ := exec.Command("fish", "--no-config", "-c", `set -x PATH `+bin+` $PATH; source `+hookFile+`; source `+hookFile+`; false; emit fish_postexec "git status"; echo "after"`).CombinedOutput()
	b, _ := os.ReadFile(log)
	if string(out) != "after\n" || !regexp.MustCompile(`^record\|--exit\|1\|--ms\|\d*\|--prev\|\|--\|git status\|\n$`).Match(b) {
		t.Fatalf("%q %q", out, b)
	}
}

func TestHookMissingBinarySilent(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not installed")
	}
	hookFile := writeHook(t, "zsh")
	out, _ := exec.Command("zsh", "-f", "-c", `PATH=/nonexistent; source `+hookFile+`; _termach_preexec x; (exit 5); _termach_precmd; echo "rc=$?"`).CombinedOutput()
	if string(out) != "rc=5\n" {
		t.Fatalf("%q", out)
	}
}
