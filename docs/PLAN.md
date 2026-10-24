# term-achievements v0.1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `term-achievements`, a Go CLI whose shell hook records each finished command as counters, unlocks 40 declarative achievements with a one-line toast, and prints `list` and `wrapped` views, without ever storing command lines.

**Architecture:** `internal/parse` turns a command line into a small in-memory `Cmd`. `internal/stats` applies an `Event` to counters, streaks and per-year aggregates. `internal/ach` holds the 40-row table and the unlock check. `internal/store` does locked, atomic JSON I/O. `internal/render` produces toast, list and wrapped text. `internal/cli` wires commands with an injected clock.

**Tech Stack:** Go (`go 1.26` in go.mod), stdlib only, GoReleaser v2, GitHub Actions.

**Spec:** `docs/SPEC.md`

## Global Constraints

- Module `github.com/dpologdvinity/term-achievements`; `go.mod` line `go 1.26`; no third-party dependencies.
- License MIT, `Copyright (c) 2026 Kaitlyn Bassford`.
- State dir: `$TERM_ACHIEVEMENTS_HOME`, else `$XDG_STATE_HOME/term-achievements` (absolute only), else `$HOME/.local/state/term-achievements`; dir `0700`, files `0600`.
- Never write a command line, argument, previous line, cwd path or untracked command name to disk (SPEC §5.4).
- `record` always exits 0, never writes to stderr, finishes in under 10 ms, runs no subprocess.
- Tests never touch the real HOME; the clock is injected; base time `t0 = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)` (a Tuesday).
- Errors: `term-achievements: <message>` on stderr; exit 1 runtime, 2 usage.
- User-facing text copied verbatim from SPEC §4.1, §7.1, §8.
- Releases: linux/darwin × amd64/arm64, `CGO_ENABLED=0`.
- Conventional Commits. Never commit `CLAUDE.md`, `.claude/`, `AGENTS.md`, `.mcp.json`, `.env`.

## Review Focus

- A password typed at the prompt by mistake (`hunter2pass` as a command): it must never reach disk, not even as a `Cmds` key (test in Task 10: `TestRecordPrivacy`).
- The clock moving backwards across midnight (NTP fix, travel): the streak must not reset or double-count (test in Task 4: `TestStreakClockBackwards`).
- Two shells finishing commands at the same moment: one event may be dropped, but the file must stay valid and the prompt must not wait more than 5 ms (test in Task 10: `TestRecordLockBusy`).
- A corrupt `stats.json`: `record` stays silent and starts fresh; `list` warns once (tests in Task 6: `TestLoadCorrupt`, Task 11: `TestListCorruptWarns`).
- Odd lines: empty, only `FOO=1`, only `sudo`, `\ls`, pipelines, `sudo -u root vim` (test in Task 2: `TestParseEdgeCases`).

---

### Task 1: Scaffold, version and help

**Files:**
- Create: `go.mod`, `main.go`, `LICENSE`, `.gitignore`, `internal/cli/cli.go`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.goreleaser.yaml`
- Test: `internal/cli/cli_test.go`

**Interfaces:**
- Produces:
  ```go
  package cli
  type Deps struct {
      Version string
      Now     func() time.Time
      Getenv  func(string) string
      Getwd   func() (string, error)
      TempDirs []string // production: os.TempDir() and "/tmp" (deduplicated)
      Stdout, Stderr io.Writer
  }
  func Run(args []string, d Deps) int
  func RealDeps(version string) Deps
  ```
  Test helper (used by Tasks 10–11): `func testDeps(t *testing.T) (Deps, *bytes.Buffer, *bytes.Buffer)` with `Now` returning `t0`, `Getenv` from a map holding `TERM_ACHIEVEMENTS_HOME=t.TempDir()` and `HOME=<another t.TempDir()>`, `Getwd` returning a third temp dir, `TempDirs` = `[]string{<a fourth temp dir>}`.

- [ ] **Step 1: Write the failing test**

```go
func TestVersionHelpUnknown(t *testing.T) {
    d, out, errb := testDeps(t); d.Version = "0.1.0"
    if Run([]string{"--version"}, d) != 0 || out.String() != "term-achievements 0.1.0\n" { t.Fatal(out) }
    out.Reset()
    if Run([]string{"help"}, d) != 0 || !strings.HasPrefix(out.String(), "term-achievements - achievements for your shell\n") { t.Fatal(out) }
    if Run([]string{"nope"}, d) != 2 || errb.String() != "term-achievements: unknown command \"nope\" (see term-achievements --help)\n" { t.Fatal(errb) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/`
Expected: FAIL (build error: `undefined: Run`)

- [ ] **Step 3: Implement**

`go mod init github.com/dpologdvinity/term-achievements`, set `go 1.26`. `main.go` with `var version = "dev"`. `Run` handles `--version`/`-V`, `--help`/`-h`/`help` (SPEC §4.1 verbatim) and unknown commands. `LICENSE` MIT with `Copyright (c) 2026 Kaitlyn Bassford`; `.gitignore`: `dist/`, `.env`.
CI (`ci.yml`): push + pull_request; matrix `[ubuntu-latest, macos-latest]`; `actions/checkout@v7`; `actions/setup-go@v7` with `go-version-file: go.mod`; ubuntu only: `sudo apt-get update && sudo apt-get install -y zsh fish`; then `test -z "$(gofmt -l .)"`, `go vet ./...`, `go test ./...`.
Release (`release.yml`): tags `v*`, `permissions: contents: write`, checkout `fetch-depth: 0`, setup-go, `goreleaser/goreleaser-action@v7` with `version: "~> v2"`, `args: release --clean`, `GITHUB_TOKEN`.
`.goreleaser.yaml`: `version: 2`, `project_name: term-achievements`, `CGO_ENABLED=0`, `goos: [linux, darwin]`, `goarch: [amd64, arm64]`, `ldflags: ["-s -w -X main.version={{.Version}}"]`, archives `formats: [tar.gz]` with `LICENSE` and `README.md`, `checksum.name_template: checksums.txt`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ && go vet ./... && test -z "$(gofmt -l .)"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod main.go LICENSE .gitignore internal/cli .github .goreleaser.yaml
git commit -m "chore: scaffold term-achievements CLI with CI and release config"
```

---

### Task 2: Command-line parsing

**Files:**
- Create: `internal/parse/parse.go`
- Test: `internal/parse/parse_test.go`

**Interfaces:**
- Produces: `type Cmd struct { Name string; Sudo bool; Args []string; Sub string; SubArgs []string; SudoRetry bool }`; `func Parse(line, prev string) Cmd`.

- [ ] **Step 1: Write the failing tests**

```go
func TestParse(t *testing.T) {
    cases := []struct{ line, prev string; want Cmd }{
        {"vim notes.md", "", Cmd{Name: "vim", Args: []string{"notes.md"}}},
        {"FOO=1 BAR=2 /usr/bin/make -j8", "", Cmd{Name: "make", Args: []string{"-j8"}}},
        {"sudo -u root vim /etc/hosts", "", Cmd{Name: "vim", Sudo: true, Args: []string{"/etc/hosts"}}},
        {"time nohup env X=1 cargo build", "", Cmd{Name: "cargo", Args: []string{"build"}}},
        {"git -C repo -c a=b --no-pager commit -m x", "", Cmd{Name: "git", Args: []string{"-C", "repo", "-c", "a=b", "--no-pager", "commit", "-m", "x"}, Sub: "commit", SubArgs: []string{"-m", "x"}}},
        {"ls -la | grep foo && echo hi", "", Cmd{Name: "ls", Args: []string{"-la"}}},
        {"\\ls", "", Cmd{Name: "ls", Args: []string{}}},
        {"sudo apt  update", "apt update", Cmd{Name: "apt", Sudo: true, Args: []string{"update"}, SudoRetry: true}},
        {"sudo apt update", "apt upgrade", Cmd{Name: "apt", Sudo: true, Args: []string{"update"}}},
    }
    for _, c := range cases {
        got := Parse(c.line, c.prev)
        if !reflect.DeepEqual(normalize(got), normalize(c.want)) { t.Errorf("%q: got %+v want %+v", c.line, got, c.want) }
    }
}
func TestParseEdgeCases(t *testing.T) {
    for _, line := range []string{"", "   ", "FOO=1", "sudo", "sudo -u root", "| grep x"} {
        if got := Parse(line, ""); got.Name != "" { t.Errorf("%q: got name %q", line, got.Name) }
    }
    if got := Parse("sudo", ""); !got.Sudo { t.Error("bare sudo still sets Sudo") }
}
```

`normalize` in the test turns nil slices into empty slices so `reflect.DeepEqual` compares content only.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/parse/`
Expected: FAIL (build error: `undefined: Parse`)

- [ ] **Step 3: Implement `Parse` per SPEC §6.1 steps 1–6**

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/parse/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/parse
git commit -m "feat(parse): extract command name, sudo, git subcommand and sudo retry"
```

---

### Task 3: Counters

**Files:**
- Create: `internal/stats/stats.go`
- Test: `internal/stats/stats_test.go`

**Interfaces:**
- Consumes: `parse.Cmd`, `parse.Parse` (Task 2).
- Produces: `Stats`, `Year` (fields and JSON tags exactly as SPEC §5.2); `func NewStats() *Stats` (Version 1, all maps non-nil); `type Event struct{ Cmd parse.Cmd; Exit int; Ms int64; Now time.Time; CwdType string }`; `func Apply(s *Stats, e Event)`; `func CwdType(cwd, home string, tmps []string) string`; `var Tracked, Typos, Toys, BuildTools map[string]bool` (SPEC §6.2 lists verbatim); `var CounterKeys []string` (every key in the SPEC §6.3 table plus `best_streak` and `distinct`).

- [ ] **Step 1: Write the failing tests**

```go
func ev(line string, exit int, ms int64, now time.Time, cwd string) Event {
    return Event{Cmd: parse.Parse(line, ""), Exit: exit, Ms: ms, Now: now, CwdType: cwd}
}
func TestCounters(t *testing.T) {
    sat8 := time.Date(2026, 3, 14, 8, 0, 0, 0, time.UTC) // Saturday 08:00
    night := time.Date(2026, 3, 10, 3, 59, 0, 0, time.UTC)
    early := time.Date(2026, 3, 10, 5, 0, 0, 0, time.UTC)
    cases := []struct{ e Event; key string; want int64 }{
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
        s := NewStats(); Apply(s, c.e)
        if got := s.Counters[c.key]; got != c.want { t.Errorf("%q exit %d: %s=%d want %d", strings.Join(append([]string{c.e.Cmd.Name}, c.e.Cmd.Args...), " "), c.e.Exit, c.key, got, c.want) }
    }
}
func TestTrackedOnly(t *testing.T) {
    s := NewStats()
    Apply(s, ev("hunter2pass", 127, 1, t0, "other"))
    Apply(s, ev("git status", 0, 1, t0, "repo"))
    if len(s.Cmds) != 1 || s.Cmds["git"] != 1 || s.Counters["distinct"] != 1 { t.Fatalf("%v", s.Cmds) }
    if y := s.Years["2026"]; y.Cmds["hunter2pass"] != 0 || y.Cmds["git"] != 1 || y.Cwd["repo"] != 1 || y.Cwd["other"] != 1 { t.Fatalf("%+v", y) }
}
func TestCwdType(t *testing.T) {
    root := t.TempDir()
    home := filepath.Join(root, "home"); tmp := filepath.Join(root, "tmp")
    repo := filepath.Join(home, "src", "p"); os.MkdirAll(filepath.Join(repo, ".git"), 0o755)
    os.MkdirAll(filepath.Join(repo, "deep"), 0o755); os.MkdirAll(filepath.Join(tmp, "x"), 0o755)
    for cwd, want := range map[string]string{home: "home", tmp: "tmp", filepath.Join(tmp, "x"): "tmp", filepath.Join(repo, "deep"): "repo", filepath.Join(home, "src"): "other"} {
        if got := CwdType(cwd, home, []string{tmp}); got != want { t.Errorf("%s: %s want %s", cwd, got, want) }
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/stats/`
Expected: FAIL (build error: `undefined: NewStats`)

- [ ] **Step 3: Implement the counter table and `Apply` steps 2 and 4 of SPEC §6.3** (FailStreak, `comebacks` and the daily streak are Task 4; this task writes the counter table, tracked names and year aggregates)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/stats/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/stats
git commit -m "feat(stats): count command features into fixed counters and yearly aggregates"
```

---

### Task 4: Streaks and comebacks

**Files:**
- Modify: `internal/stats/stats.go`
- Test: `internal/stats/streak_test.go`

**Interfaces:**
- Consumes: `Apply`, `NewStats`, `ev` helper (Task 3).
- Produces: `Apply` now also maintains `FailStreak`, `Streak`, `LastDay`, `Counters["best_streak"]`, `Counters["comebacks"]`, `Year.ActiveDays`, `Year.BestStreak` (SPEC §6.3 steps 1 and 3).

- [ ] **Step 1: Write the failing tests**

```go
func day(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 0, 0, 0, time.UTC) }

func TestStreak(t *testing.T) {
    s := NewStats()
    for d := 1; d <= 3; d++ { Apply(s, ev("ls", 0, 1, day(2026, 3, d, 10), "other")); Apply(s, ev("ls", 0, 1, day(2026, 3, d, 11), "other")) }
    if s.Streak != 3 || s.Counters["best_streak"] != 3 || s.Years["2026"].ActiveDays != 3 { t.Fatalf("%+v", s) }
    Apply(s, ev("ls", 0, 1, day(2026, 3, 5, 10), "other")) // gap
    if s.Streak != 1 || s.Counters["best_streak"] != 3 { t.Fatal(s.Streak) }
}
func TestStreakAcrossNewYear(t *testing.T) {
    s := NewStats()
    Apply(s, ev("ls", 0, 1, day(2025, 12, 31, 23), "other"))
    Apply(s, ev("ls", 0, 1, day(2026, 1, 1, 0), "other"))
    if s.Streak != 2 || s.Years["2026"].BestStreak != 2 || s.Years["2025"].BestStreak != 1 { t.Fatalf("%+v", s) }
}
func TestStreakClockBackwards(t *testing.T) {
    s := NewStats()
    Apply(s, ev("ls", 0, 1, day(2026, 3, 2, 0), "other"))
    Apply(s, ev("ls", 0, 1, day(2026, 3, 1, 23), "other"))
    if s.Streak != 1 || s.LastDay != "2026-03-02" || s.Years["2026"].ActiveDays != 1 { t.Fatalf("%+v", s) }
    if s.Years["2026"].Commands != 2 { t.Fatal("the event itself still counts") }
}
func TestComeback(t *testing.T) {
    s := NewStats()
    for i := 0; i < 10; i++ { Apply(s, ev("make", 2, 1, t0, "other")); if i == 4 { Apply(s, ev("sleep 1", 130, 1, t0, "other")) } }
    Apply(s, ev("make", 0, 1, t0, "other"))
    if s.Counters["comebacks"] != 1 || s.FailStreak != 0 { t.Fatalf("%d %d", s.Counters["comebacks"], s.FailStreak) }
    for i := 0; i < 9; i++ { Apply(s, ev("make", 2, 1, t0, "other")) }
    Apply(s, ev("make", 0, 1, t0, "other"))
    if s.Counters["comebacks"] != 1 { t.Fatal("9 failures must not count") }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/stats/ -run 'Streak|Comeback'`
Expected: FAIL (`Streak` stays 0)

- [ ] **Step 3: Implement SPEC §6.3 steps 1 and 3** (the `comebacks` check happens before FailStreak resets)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/stats/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/stats
git commit -m "feat(stats): track daily streaks and comebacks after failure runs"
```

---

### Task 5: Achievement table and unlocking

**Files:**
- Create: `internal/ach/ach.go`
- Test: `internal/ach/ach_test.go`

**Interfaces:**
- Consumes: `stats.Stats`, `stats.CounterKeys`, `stats.NewStats` (Tasks 3–4).
- Produces: `type Achievement struct{ ID, Title, Desc, Counter string; Goal int64 }`; `var All []Achievement` (SPEC §7.1, 40 rows, same order, text verbatim); `func Evaluate(s *stats.Stats, now time.Time) []Achievement`.

- [ ] **Step 1: Write the failing tests**

```go
func TestTableInvariants(t *testing.T) {
    if len(All) != 40 { t.Fatal(len(All)) }
    keys := map[string]bool{}; for _, k := range stats.CounterKeys { keys[k] = true }
    seen := map[string]bool{}
    idRe := regexp.MustCompile(`^[a-z0-9_]+$`)
    for _, a := range All {
        if seen[a.ID] || !idRe.MatchString(a.ID) { t.Errorf("bad id %q", a.ID) }; seen[a.ID] = true
        if !keys[a.Counter] { t.Errorf("%s: unknown counter %q", a.ID, a.Counter) }
        if a.Goal < 1 || utf8.RuneCountInString(a.Title) > 26 || utf8.RuneCountInString(a.Desc) > 44 { t.Errorf("%s: bad goal or length", a.ID) }
    }
    if All[5] != (Achievement{"escaped_vim", "Escaped Vim", "exit vim 10 times", "vim", 10}) { t.Fatalf("%+v", All[5]) }
    if All[39].ID != "toolbox" { t.Fatal(All[39]) }
}
func TestEvaluate(t *testing.T) {
    s := stats.NewStats()
    s.Counters["commands"] = 100; s.Counters["vim"] = 9
    got := Evaluate(s, t0)
    if len(got) != 2 || got[0].ID != "first_steps" || got[1].ID != "warming_up" || !s.Unlocked["warming_up"].Equal(t0) { t.Fatal(got) }
    if again := Evaluate(s, t0.Add(time.Hour)); len(again) != 0 { t.Fatal("unlocked twice") }
    s.Counters["vim"] = 10
    if got := Evaluate(s, t0); len(got) != 1 || got[0].Title != "Escaped Vim" { t.Fatal(got) }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ach/`
Expected: FAIL (build error: `undefined: All`)

- [ ] **Step 3: Write the table verbatim from SPEC §7.1 and implement `Evaluate` per §7.2**

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ach/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ach
git commit -m "feat(ach): add the 40-achievement table and unlock evaluation"
```

---

### Task 6: Store

**Files:**
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Consumes: `stats.Stats`, `stats.NewStats` (Task 3).
- Produces:
  ```go
  var ErrBusy = errors.New("another term-achievements is busy; try again")
  var ErrNewer = errors.New("stats were written by a newer term-achievements; please upgrade")
  func Dir(getenv func(string) string) (string, error)
  func Lock(dir string, shared bool, budget time.Duration) (unlock func(), err error) // stats.lock; LOCK_EX or LOCK_SH, LOCK_NB every 500µs
  func Load(dir string, now time.Time) (s *stats.Stats, movedTo string, err error)
  func Save(dir string, s *stats.Stats) error // atomic, 0600, creates dir 0700
  ```

- [ ] **Step 1: Write the failing tests**

```go
func TestDir(t *testing.T) {
    g := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
    if d, _ := Dir(g(map[string]string{"TERM_ACHIEVEMENTS_HOME": "/a", "HOME": "/h"})); d != "/a" { t.Fatal(d) }
    if d, _ := Dir(g(map[string]string{"XDG_STATE_HOME": "/s", "HOME": "/h"})); d != "/s/term-achievements" { t.Fatal(d) }
    if d, _ := Dir(g(map[string]string{"XDG_STATE_HOME": "s", "HOME": "/h"})); d != "/h/.local/state/term-achievements" { t.Fatal(d) }
}
func TestRoundTrip(t *testing.T) {
    dir := filepath.Join(t.TempDir(), "ta")
    s := stats.NewStats(); s.Counters["vim"] = 3; s.Unlocked["first_steps"] = t0
    if err := Save(dir, s); err != nil { t.Fatal(err) }
    got, moved, err := Load(dir, t0)
    if err != nil || moved != "" || got.Counters["vim"] != 3 || !got.Unlocked["first_steps"].Equal(t0) { t.Fatal(got, err) }
    if fi, _ := os.Stat(filepath.Join(dir, "stats.json")); fi.Mode().Perm() != 0o600 { t.Fatal(fi.Mode()) }
}
func TestLoadCorrupt(t *testing.T) {
    dir := t.TempDir(); os.WriteFile(filepath.Join(dir, "stats.json"), []byte("{\"counters\":"), 0o600)
    s, moved, err := Load(dir, t0)
    if err != nil || len(s.Counters) != 0 || moved != filepath.Join(dir, fmt.Sprintf("stats.json.corrupt-%d", t0.Unix())) { t.Fatal(s, moved, err) }
}
func TestLoadNewer(t *testing.T) {
    dir := t.TempDir(); os.WriteFile(filepath.Join(dir, "stats.json"), []byte(`{"version":2}`), 0o600)
    if _, _, err := Load(dir, t0); !errors.Is(err, ErrNewer) { t.Fatal(err) }
}
func TestLockBusy(t *testing.T) {
    dir := t.TempDir()
    un, _ := Lock(dir, false, time.Second); defer un()
    s := time.Now()
    if _, err := Lock(dir, false, 5*time.Millisecond); !errors.Is(err, ErrBusy) || time.Since(s) > 50*time.Millisecond { t.Fatal(err) }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/`
Expected: FAIL (build error: `undefined: Dir`)

- [ ] **Step 3: Implement per SPEC §5.1 and §5.3**

Loaded maps that are `nil` in JSON become empty maps (so callers never nil-check). `Dir` errors with `cannot find a home directory` when nothing is set.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store
git commit -m "feat(store): add locked atomic stats storage with corrupt-file recovery"
```

---

### Task 7: Toasts and list

**Files:**
- Create: `internal/render/toast.go`, `internal/render/list.go`
- Test: `internal/render/render_test.go`

**Interfaces:**
- Consumes: `ach.Achievement`, `ach.All` (Task 5); `stats.Stats` (Task 3).
- Produces: `func Toasts(unlocked []ach.Achievement) string` (each line ends in `\n`; `""` for none); `func List(s *stats.Stats, loc *time.Location) string`.

- [ ] **Step 1: Write the failing tests**

```go
func TestToasts(t *testing.T) {
    if Toasts(nil) != "" { t.Fatal() }
    one := Toasts(ach.All[5:6])
    if one != "🏆 Achievement unlocked: Escaped Vim — exit vim 10 times\n" { t.Fatalf("%q", one) }
    four := Toasts(ach.All[0:4])
    want := "🏆 Achievement unlocked: First Steps — run your first command\n" +
        "🏆 Achievement unlocked: Warming Up — run 100 commands\n" +
        "🏆 …and 2 more (see term-achievements list)\n"
    if four != want { t.Fatalf("%q", four) }
    if three := Toasts(ach.All[0:3]); strings.Count(three, "\n") != 3 || strings.Contains(three, "more") { t.Fatal(three) }
}
func TestList(t *testing.T) {
    s := stats.NewStats()
    s.Counters["commands"] = 37; s.Unlocked["first_steps"] = t0
    out := List(s, time.UTC)
    lines := strings.Split(out, "\n")
    if lines[0] != "term-achievements · 1/40 unlocked" || lines[1] != "" { t.Fatal(lines[:2]) }
    if lines[2] != "✓ First Steps                run your first command                       2026-03-10" { t.Fatalf("%q", lines[2]) }
    if lines[3] != "· Warming Up                 run 100 commands                             [###-------] 37/100" { t.Fatalf("%q", lines[3]) }
    if len(lines) != 43 || lines[42] != "" { t.Fatal(len(lines)) } // 2 header + 40 rows + trailing newline
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/`
Expected: FAIL (build error: `undefined: Toasts`)

- [ ] **Step 3: Implement per SPEC §8.1 and §8.2**

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/render
git commit -m "feat(render): add unlock toasts and the achievement list"
```

---

### Task 8: Wrapped card

**Files:**
- Create: `internal/render/wrapped.go`
- Test: `internal/render/wrapped_test.go`

**Interfaces:**
- Consumes: `stats.Stats`, `stats.Year` (Task 3), `ach.All` (Task 5).
- Produces: `func Wrapped(s *stats.Stats, year int) (string, bool)` (false when the year has no commands); helpers `commas(n int64) string`, `dur(ms int64) string`.

- [ ] **Step 1: Write the failing tests**

```go
func TestWrappedFull(t *testing.T) {
    s := stats.NewStats()
    y := &stats.Year{Commands: 12345, Failures: 383, Commits: 412, ActiveDays: 201, BestStreak: 14, LongestMs: 23*60000 + 10500,
        Cmds: map[string]int64{"git": 3210, "ls": 1998, "vim": 1998, "cd": 50, "make": 7, "rg": 6},
        Cwd: map[string]int64{"repo": 7778, "home": 4567}}
    y.Hours[14] = 900; y.Hours[15] = 900
    s.Years["2026"] = y
    for _, id := range []string{"first_steps", "warming_up", "keyboard_warrior", "shell_dweller", "escaped_vim", "night_owl", "rtfm"} { s.Unlocked[id] = t0 }
    s.Unlocked["snapshot"] = time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC) // other year: excluded
    got, ok := Wrapped(s, 2026)
    want := strings.Join([]string{
        "╭────────────────────────────────────────────╮",
        "│ term-achievements wrapped · 2026           │",
        "│                                            │",
        "│ 12,345 commands on 201 days                │",
        "│ 3.1% failed · best streak 14 days          │",
        "│ busiest hour 14:00-15:00                   │",
        "│ longest command 23m 10s                    │",
        "│ 412 git commits                            │",
        "│ mostly in: git repos (63%)                 │",
        "│                                            │",
        "│ top commands                               │",
        "│   1. git        3,210                      │",
        "│   2. ls         1,998                      │",
        "│   3. vim        1,998                      │",
        "│   4. cd         50                         │",
        "│   5. make       7                          │",
        "│                                            │",
        "│ 7 achievements unlocked                    │",
        "│   First Steps, Warming Up,                 │",
        "│   Keyboard Warrior, Shell Dweller,         │",
        "│   Escaped Vim, RTFM, Night Owl             │",
        "╰────────────────────────────────────────────╯",
    }, "\n") + "\n"
    if !ok || got != want { t.Fatalf("got\n%s\nwant\n%s", got, want) }
}
func TestWrappedEmptyAndHelpers(t *testing.T) {
    if _, ok := Wrapped(stats.NewStats(), 2026); ok { t.Fatal("empty year") }
    for ms, want := range map[int64]string{0: "0s", 59999: "59s", 60000: "1m 0s", 3600000: "1h 0m", 3*3600000 + 125000: "3h 2m"} {
        if got := dur(ms); got != want { t.Errorf("%d: %s", ms, got) }
    }
    if commas(1234567) != "1,234,567" || commas(999) != "999" { t.Fatal() }
}
func TestWrappedSingular(t *testing.T) {
    s := stats.NewStats()
    s.Years["2026"] = &stats.Year{Commands: 1, ActiveDays: 1, BestStreak: 1, Commits: 1, Cwd: map[string]int64{"tmp": 1}, Cmds: map[string]int64{}}
    got, _ := Wrapped(s, 2026)
    for _, want := range []string{"│ 1 command on 1 day ", "│ 0.0% failed · best streak 1 day ", "│ 1 git commit ", "│ mostly in: temp dirs (100%) ", "│ busiest hour 00:00-01:00 ", "│   (none tracked yet) ", "│ 0 achievements unlocked "} {
        if !strings.Contains(got, want) { t.Errorf("missing %q in\n%s", want, got) }
    }
}
```

Note on the expected wrap: `  First Steps, Warming Up, Keyboard Warrior,` would be 44 runes (> 42), so `Keyboard Warrior,` starts a new line.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/ -run Wrapped`
Expected: FAIL (build error: `undefined: Wrapped`)

- [ ] **Step 3: Implement per SPEC §8.3**

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/render
git commit -m "feat(render): add the yearly wrapped card"
```

---

### Task 9: Shell hooks

**Files:**
- Create: `internal/hooks/hooks.go`, `internal/hooks/term-achievements.zsh`, `internal/hooks/term-achievements.bash`, `internal/hooks/term-achievements.fish`
- Test: `internal/hooks/hooks_test.go`

**Interfaces:**
- Produces: `func Script(shell string) (string, bool)`.

- [ ] **Step 1: Write the failing tests**

Helper `fakeBin(t) (binDir, logPath string)`: writes an executable `term-achievements` that runs `printf '%s|' "$@" >> <log>; echo >> <log>`.

```go
func TestBashHookInteractive(t *testing.T) {
    hookFile := writeHook(t, "bash")
    bin, log := fakeBin(t)
    cmd := exec.Command("bash", "--norc", "-i")
    cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir(), "HISTFILE=/dev/null", "TERM=dumb"}
    cmd.Stdin = strings.NewReader("source " + hookFile + "\nfalse\ntrue  x\n\n")
    cmd.Run()
    b, _ := os.ReadFile(log)
    lines := strings.Split(strings.TrimSpace(string(b)), "\n")
    if len(lines) != 2 { t.Fatalf("%q", b) }
    if !regexp.MustCompile(`^record\|--exit\|1\|--started\|\d+\|--prev\|\|--\|false\|$`).MatchString(lines[0]) { t.Fatal(lines[0]) }
    if !regexp.MustCompile(`^record\|--exit\|0\|--started\|\d+\|--prev\|false\|--\|true  x\|$`).MatchString(lines[1]) { t.Fatal(lines[1]) }
}
func TestZshHook(t *testing.T) {
    if _, err := exec.LookPath("zsh"); err != nil { t.Skip("zsh not installed") }
    hookFile := writeHook(t, "zsh"); bin, log := fakeBin(t)
    out, err := exec.Command("zsh", "-f", "-c", `PATH=`+bin+`:$PATH; source `+hookFile+`; source `+hookFile+`; _termach_preexec "make all"; (exit 3); _termach_precmd; echo "rc=$?"; _termach_precmd; echo "rc=$?"`).CombinedOutput()
    if string(out) != "rc=3\nrc=0\n" || err != nil { t.Fatalf("%q %v", out, err) }
    b, _ := os.ReadFile(log)
    if !regexp.MustCompile(`^record\|--exit\|3\|--ms\|\d+\|--prev\|\|--\|make all\|\n$`).Match(b) { t.Fatalf("%q", b) } // second precmd records nothing
}
func TestFishHook(t *testing.T) {
    if _, err := exec.LookPath("fish"); err != nil { t.Skip("fish not installed") }
    hookFile := writeHook(t, "fish"); bin, log := fakeBin(t)
    out, _ := exec.Command("fish", "--no-config", "-c", `set -x PATH `+bin+` $PATH; source `+hookFile+`; source `+hookFile+`; false; emit fish_postexec "git status"; echo "after"`).CombinedOutput()
    b, _ := os.ReadFile(log)
    if string(out) != "after\n" || !regexp.MustCompile(`^record\|--exit\|1\|--ms\|\d*\|--prev\|\|--\|git status\|\n$`).Match(b) { t.Fatalf("%q %q", out, b) }
}
func TestHookMissingBinarySilent(t *testing.T) {
    hookFile := writeHook(t, "zsh")
    if _, err := exec.LookPath("zsh"); err != nil { t.Skip() }
    out, _ := exec.Command("zsh", "-f", "-c", `PATH=/nonexistent; source `+hookFile+`; _termach_preexec x; (exit 5); _termach_precmd; echo "rc=$?"`).CombinedOutput()
    if string(out) != "rc=5\n" { t.Fatalf("%q", out) }
}
```

`writeHook(t, shell)` writes `Script(shell)` into a temp file and returns its path. In `TestHookMissingBinarySilent` the zsh binary is started by absolute path from `exec.Command`, so `PATH=/nonexistent` only affects the hook. `Script("tcsh")` returns `"", false`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/hooks/`
Expected: FAIL (build error: `undefined: Script`)

- [ ] **Step 3: Write the hook files verbatim from SPEC §9; `Script` serves them via `//go:embed`**

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/hooks/`
Expected: PASS (zsh/fish tests PASS or SKIP when not installed; all run in CI)

- [ ] **Step 5: Commit**

```bash
git add internal/hooks
git commit -m "feat(hooks): add zsh, bash and fish hooks that report finished commands"
```

---

### Task 10: `record`

**Files:**
- Create: `internal/cli/record.go`
- Modify: `internal/cli/cli.go` (dispatch `record`)
- Test: `internal/cli/record_test.go`

**Interfaces:**
- Consumes: `parse.Parse`, `stats.Apply`, `stats.CwdType`, `stats.Event`, `ach.Evaluate`, `store.*`, `render.Toasts`, `Deps`, `testDeps`.
- Produces: `func runRecord(args []string, d Deps) int` (always 0).

- [ ] **Step 1: Write the failing tests**

```go
func rec(d Deps, exit string, line ...string) { Run(append([]string{"record", "--exit", exit, "--ms", "5", "--"}, line...), d) }

func TestRecordFirstToast(t *testing.T) {
    d, out, errb := testDeps(t)
    rec(d, "0", "ls", "-la")
    if out.String() != "🏆 Achievement unlocked: First Steps — run your first command\n" || errb.Len() != 0 { t.Fatalf("%q %q", out, errb) }
    out.Reset(); rec(d, "0", "ls")
    if out.Len() != 0 { t.Fatal(out) }
}
func TestRecordEscapedVim(t *testing.T) {
    d, out, _ := testDeps(t)
    for i := 0; i < 10; i++ { out.Reset(); rec(d, "0", "vim", "x") }
    if out.String() != "🏆 Achievement unlocked: Escaped Vim — exit vim 10 times\n" { t.Fatalf("%q", out) }
}
func TestRecordStartedAndPrev(t *testing.T) {
    d, _, _ := testDeps(t)
    Run([]string{"record", "--exit", "0", "--started", strconv.FormatInt(t0.Unix()-700, 10), "--", "make"}, d)
    Run([]string{"record", "--exit", "0", "--prev", "apt update", "--", "sudo", "apt", "update"}, d)
    s, _, _ := store.Load(d.Getenv("TERM_ACHIEVEMENTS_HOME"), t0)
    if s.Counters["long_build"] != 1 || s.Counters["sudo_retry"] != 1 { t.Fatalf("%v", s.Counters) }
}
func TestRecordBadInputSilent(t *testing.T) {
    d, out, errb := testDeps(t)
    for _, args := range [][]string{{"record"}, {"record", "--exit", "x", "--", "ls"}, {"record", "--exit", "0", "--"}, {"record", "--bogus"}} {
        if Run(args, d) != 0 || out.Len()+errb.Len() != 0 { t.Fatalf("%v: %q %q", args, out, errb) }
    }
}
func TestRecordDisabled(t *testing.T) {
    d, out, _ := testDeps(t); d.Getenv = withEnv(d.Getenv, "TERM_ACHIEVEMENTS_DISABLE", "1")
    rec(d, "0", "ls")
    if out.Len() != 0 { t.Fatal(out) }
    if _, err := os.Stat(filepath.Join(d.Getenv("TERM_ACHIEVEMENTS_HOME"), "stats.json")); err == nil { t.Fatal("wrote state") }
}
func TestRecordPrivacy(t *testing.T) {
    d, _, _ := testDeps(t)
    secretDir := filepath.Join(t.TempDir(), "secretproject"); os.MkdirAll(secretDir, 0o755)
    d.Getwd = func() (string, error) { return secretDir, nil }
    prev := []string{"record", "--exit", "0", "--prev", "export API_KEY=zzsecretzz", "--"}
    Run(append(prev, "git", "commit", "-m", `"SECRET_TOKEN_abc123"`), d)
    Run(append(prev, "mysql", "-phunter2pass"), d)
    Run(append(prev, "hunter2pass"), d)
    dir := d.Getenv("TERM_ACHIEVEMENTS_HOME")
    filepath.WalkDir(dir, func(p string, e fs.DirEntry, _ error) error {
        if e.IsDir() { return nil }
        b, _ := os.ReadFile(p)
        for _, bad := range []string{"SECRET_TOKEN_abc123", "hunter2pass", "zzsecretzz", "secretproject", "-m", "mysql"} {
            if bytes.Contains(b, []byte(bad)) { t.Errorf("%s contains %q", p, bad) }
        }
        return nil
    })
}
func TestRecordLockBusy(t *testing.T) {
    d, out, _ := testDeps(t)
    un, _ := store.Lock(d.Getenv("TERM_ACHIEVEMENTS_HOME"), false, time.Second); defer un()
    s := time.Now(); rec(d, "0", "ls")
    if time.Since(s) > 50*time.Millisecond || out.Len() != 0 { t.Fatal("must drop the event fast and silently") }
}
func TestRecordBudget(t *testing.T) {
    if testing.Short() { t.Skip() }
    d, _, _ := testDeps(t); d.Stdout = io.Discard
    for i := 0; i < 5000; i++ { rec(d, strconv.Itoa(i%3), "git", "status") } // a busy history
    var ts []time.Duration
    for i := 0; i < 20; i++ { s := time.Now(); rec(d, "0", "ls"); ts = append(ts, time.Since(s)) }
    slices.Sort(ts)
    if ts[10] >= 10*time.Millisecond { t.Fatalf("median %v", ts[10]) }
}
func BenchmarkRecord(b *testing.B) { /* testDeps-equivalent with b.TempDir(); loop rec(d, "0", "ls") */ }
```

`mysql` is not a tracked name, so it must not appear on disk either. `withEnv(getenv, k, v)` is a helper next to `testDeps`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run Record`
Expected: FAIL (`unknown command "record"`)

- [ ] **Step 3: Implement `runRecord` per SPEC §6 steps 1–5**

Use a `flag.FlagSet` with `ContinueOnError` and `SetOutput(io.Discard)`; `--exit` is required (missing → return 0). `Ms` comes from `--ms`, else from `--started` (`max(0, now.UnixMilli() - started*1000)`), else 0. `cwd` from `d.Getwd` (error → `other`), home from `d.Getenv("HOME")`, tmps from `d.TempDirs`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ && go test -bench Record -run '^$' ./internal/cli/`
Expected: PASS; benchmark well under 10 ms/op

- [ ] **Step 5: Commit**

```bash
git add internal/cli
git commit -m "feat(cli): add the record command with toasts and privacy guarantees"
```

---

### Task 11: `list`, `wrapped`, `init`

**Files:**
- Modify: `internal/cli/cli.go`
- Test: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `render.List`, `render.Wrapped`, `hooks.Script`, `store.*`.
- Produces: dispatch for no args and `list`, `wrapped [--year Y]`, `init <shell>`.

- [ ] **Step 1: Write the failing tests**

```go
func TestListCommand(t *testing.T) {
    d, out, _ := testDeps(t)
    if Run(nil, d) != 0 || !strings.HasPrefix(out.String(), "term-achievements · 0/40 unlocked\n\n") { t.Fatal(out) }
    rec(d, "0", "ls"); out.Reset()
    if Run([]string{"list"}, d) != 0 || !strings.Contains(out.String(), "✓ First Steps") { t.Fatal(out) }
}
func TestListCorruptWarns(t *testing.T) {
    d, _, errb := testDeps(t)
    os.WriteFile(filepath.Join(d.Getenv("TERM_ACHIEVEMENTS_HOME"), "stats.json"), []byte("nope"), 0o600)
    if Run([]string{"list"}, d) != 0 || !strings.HasPrefix(errb.String(), "term-achievements: stats file was unreadable; moved it to ") { t.Fatal(errb) }
}
func TestWrappedCommand(t *testing.T) {
    d, out, errb := testDeps(t)
    if Run([]string{"wrapped"}, d) != 0 || out.String() != "no commands recorded in 2026 yet\n" { t.Fatal(out) }
    rec(d, "0", "git", "status"); out.Reset()
    if Run([]string{"wrapped", "--year", "2026"}, d) != 0 || !strings.Contains(out.String(), "│ 1 command on 1 day") { t.Fatal(out) }
    if Run([]string{"wrapped", "--year", "twenty"}, d) != 2 || errb.String() != "term-achievements: --year must be a year like 2026\n" { t.Fatal(errb) }
}
func TestInitCommand(t *testing.T) {
    d, out, errb := testDeps(t)
    want, _ := hooks.Script("bash")
    if Run([]string{"init", "bash"}, d) != 0 || out.String() != want { t.Fatal(out) }
    if Run([]string{"init"}, d) != 2 || errb.String() != "term-achievements: init needs a shell: zsh, bash or fish\n" { t.Fatal(errb) }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/`
Expected: FAIL (`unknown command "list"`)

- [ ] **Step 3: Implement**

`list`/`wrapped` take `store.Lock(dir, true, 2*time.Second)` (busy → exit 1 with `term-achievements: ` + `ErrBusy`), `store.Load`, print the corrupt warning if `movedTo != ""`, and save only when a corrupt file was moved (so the fresh empty file exists). `--year` accepts integers 1970–9999 only. Location for list dates: `d.Now().Location()`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli
git commit -m "feat(cli): add list, wrapped and init commands"
```

---

### Task 12: README and final verification

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README.md** with: pitch plus a sample toast; Install (`go install github.com/dpologdvinity/term-achievements@latest`, release archives); Setup per shell (SPEC §4.1 lines; "add it at the end of your rc file"); a tip `alias ach=term-achievements` (not shipped); Commands with a real `list` excerpt and a real `wrapped` card copied from your own run; Privacy (SPEC §5.4, in plain words, plus the tracked-name rule); bash notes (history-based, 1-second durations, `ignorespace`/`ignoredups` commands not counted); Environment (`TERM_ACHIEVEMENTS_HOME`, `TERM_ACHIEVEMENTS_DISABLE`); the full achievement list (Title — Desc) generated from `ach.All`; License MIT.

- [ ] **Step 2: Full verification**

Run: `test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build -o /tmp/term-achievements . && TERM_ACHIEVEMENTS_HOME=$(mktemp -d) /tmp/term-achievements list | head -3`
Expected: all pass; output starts with `term-achievements · 0/40 unlocked`.

- [ ] **Step 3: Manual check (SPEC §12)**

In `zsh -f` with `TERM_ACHIEVEMENTS_HOME=$(mktemp -d)` and `/tmp` on `PATH`: `eval "$(term-achievements init zsh)"`, then run `vim -c q` ten times. Expected: `🏆 Achievement unlocked: First Steps — run your first command` after the first and `🏆 Achievement unlocked: Escaped Vim — exit vim 10 times` after the tenth. Repeat once in `bash --norc` with the bash hook.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: add README with setup, privacy notes and achievement list"
```
