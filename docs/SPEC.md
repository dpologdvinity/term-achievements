# term-achievements — Specification (v0.1)

`term-achievements` is a single-binary CLI that gives your shell Xbox-style
achievements. A small zsh, bash or fish hook reports each finished command to
`term-achievements record`, which updates local counters, unlocks achievements
and prints a one-line toast before your next prompt:

```
🏆 Achievement unlocked: Escaped Vim — exit vim 10 times
```

`term-achievements list` shows every achievement with progress, and
`term-achievements wrapped` prints a shareable yearly recap card.

This document is the source of truth for v0.1 behavior. `docs/PLAN.md` says how
to build it. If the plan and the spec disagree, the spec wins.

## 1. Goals and non-goals

### Goals (v0.1)
- Hooks for zsh, bash and fish via `term-achievements init <shell>`.
- `record` runs on every prompt in under 10 ms, with no network and no git calls.
- 40 achievements defined in one declarative Go table (§7), each unlocked by one counter reaching a goal.
- `list` with progress, `wrapped [--year Y]` recap card.
- Privacy by design: only counters and aggregates are stored, never command lines or arguments (§5.4).

### Non-goals (v0.1)
- No network, no sync, no sharing service. "Shareable" means you copy the card text.
- No full shell parsing. Tokenizing is whitespace splitting (§6.1); quoting is not interpreted.
- No color, no sounds, no desktop notifications.
- No Windows build (shell hooks plus `syscall.Flock`).
- No shipped `ach` alias. The README suggests `alias ach=term-achievements`.

## 2. Name availability (checked 2026-10-23)

| Place | Result |
|---|---|
| GitHub `dpologdvinity/term-achievements` | free (404) |
| Homebrew formula / cask | free (404 / 404) |
| npm `term-achievements` | free (404) |
| GitHub repos with the name | 0 results |

## 3. Tech stack (fixed)

| Item | Choice |
|---|---|
| Language | Go. `go.mod` says `go 1.26` (builds with the installed 1.27.3). |
| Module path | `github.com/dpologdvinity/term-achievements` |
| Dependencies | none. Stdlib only (`flag`, `encoding/json`, `syscall`, `embed`, `unicode/utf8`). |
| Tests | stdlib `testing` only |
| Release | GoReleaser v2, linux/darwin × amd64/arm64, `CGO_ENABLED=0` |
| CI | GitHub Actions: `actions/checkout@v7`, `actions/setup-go@v7`, `goreleaser/goreleaser-action@v7` with `version: "~> v2"` (latest tags on 2026-10-23: v7.0.1, v7.0.0, v7.3.0; GoReleaser v2.19.0) |
| License | MIT, `Copyright (c) 2026 Kaitlyn Bassford` |

No third-party module is needed: the input is one argv list, the state is one
JSON file, and the output is plain text. Add no dependencies in v0.1.

## 4. CLI interface

```
term-achievements                         same as `list`
term-achievements list                    all achievements with progress
term-achievements wrapped [--year Y]      yearly recap card (default: current year)
term-achievements init <shell>            print the hook for zsh, bash or fish
term-achievements record [flags] -- LINE  record one finished command (used by the hook)
term-achievements --version | -V          print "term-achievements <version>"
term-achievements --help | -h | help      print usage
```

`record` flags: `--exit N` (required; integer), `--ms N` (duration in ms) or
`--started UNIX` (start time in Unix seconds; duration = now − started, floored
at 0), `--prev LINE` (the previous command line, may be empty). Everything after
`--` is joined with single spaces into LINE.

### 4.1 Usage text (exact)

```
term-achievements - achievements for your shell

usage:
  term-achievements list                 show achievements and progress
  term-achievements wrapped [--year Y]   your year in the terminal
  term-achievements init <shell>         print the shell hook (zsh, bash, fish)
  term-achievements --version            print the version

Setup (zsh):  add  eval "$(term-achievements init zsh)"  to ~/.zshrc
Setup (bash): add  eval "$(term-achievements init bash)"  to ~/.bashrc
Setup (fish): add  term-achievements init fish | source  to ~/.config/fish/config.fish

Only counters are stored. Command lines and arguments never touch the disk.
```

### 4.2 Exit codes

| Code | When |
|---|---|
| 0 | Success. `record` **always** exits 0 and never writes to stderr. |
| 1 | Runtime error (state unreadable for `list`/`wrapped`, lock busy, newer state version). |
| 2 | Usage error. |

Errors: `term-achievements: <message>` on stderr. Usage errors (exact):
- unknown command: `term-achievements: unknown command "<x>" (see term-achievements --help)`
- `init` with no or bad shell: `term-achievements: init needs a shell: zsh, bash or fish`
- bad `--year`: `term-achievements: --year must be a year like 2026`

## 5. State

### 5.1 Location
1. `$TERM_ACHIEVEMENTS_HOME` if set and non-empty (tests always set this).
2. `$XDG_STATE_HOME/term-achievements` if `XDG_STATE_HOME` is an absolute path.
3. `$HOME/.local/state/term-achievements`.

Dir mode `0700`, file mode `0600`. Files: `stats.json`, `stats.lock`.

### 5.2 `stats.json`

```go
type Stats struct {
    Version    int                  `json:"version"`     // 1
    Counters   map[string]int64     `json:"counters"`    // keys from §6.3 only
    Cmds       map[string]int64     `json:"cmds"`        // tracked names (§6.2) only
    LastDay    string               `json:"last_day"`    // "2006-01-02", local
    Streak     int64                `json:"streak"`      // current daily streak
    FailStreak int64                `json:"fail_streak"` // consecutive failures
    Unlocked   map[string]time.Time `json:"unlocked"`    // achievement ID -> unlock time
    Years      map[string]*Year     `json:"years"`       // "2026" -> aggregates
}

type Year struct {
    Commands   int64            `json:"commands"`
    Failures   int64            `json:"failures"`
    Commits    int64            `json:"commits"`
    ActiveDays int64            `json:"active_days"`
    BestStreak int64            `json:"best_streak"`
    LongestMs  int64            `json:"longest_ms"`
    Hours      [24]int64        `json:"hours"`
    Cmds       map[string]int64 `json:"cmds"`  // tracked names only
    Cwd        map[string]int64 `json:"cwd"`   // "repo" | "home" | "tmp" | "other"
}
```

### 5.3 Load, save, lock
Same rules as a small key-value file:
- Read-modify-write holds `syscall.Flock(LOCK_EX)` on `stats.lock`. `record` tries `LOCK_NB` every 500 µs for at most **5 ms**; on timeout it drops this one event and exits 0 silently. `list` and `wrapped` take a shared lock (`LOCK_SH`) with a 2 s budget, then exit 1 with `term-achievements: another term-achievements is busy; try again`.
- Save is atomic (`stats.json.tmp-<pid>` then `os.Rename`).
- Missing file: empty stats (all maps empty, Version 1).
- Unparseable JSON: rename to `stats.json.corrupt-<unix seconds>` and start empty. `list`/`wrapped` print `term-achievements: stats file was unreadable; moved it to <path> and started fresh` to stderr (exit 0). `record` stays silent.
- `version` > 1: `record` does nothing; `list`/`wrapped` exit 1 with `term-achievements: stats were written by a newer term-achievements; please upgrade`.

### 5.4 Privacy rules (tested)
- The command line, the previous command line, every argument, and the cwd path are used only in memory during one `record` run. None of them is written anywhere.
- A command name is stored (in `Cmds` or `Year.Cmds`) only if it is in the tracked-name list (§6.2). Anything else (including a password mistakenly typed as a command) only increments generic counters.
- Counter keys are the fixed set in §6.3. No key is ever built from user input.
- A test records lines containing secrets and asserts no file in the state dir contains them (§11).

## 6. Recording (`record`, the hot path)

Budget: **under 10 ms** per call, no network, no subprocesses, no git. Enforced
by `TestRecordBudget` (median of 20 runs < 10 ms, skipped with `-short`) and
`BenchmarkRecord`.

Steps:
1. If `TERM_ACHIEVEMENTS_DISABLE` is non-empty, exit 0.
2. Parse flags; on any error exit 0. Parse LINE (§6.1). If there is no command name, exit 0.
3. Build the event (§6.1–6.2) with `now` from the injected clock and `cwdType` (§6.2).
4. Lock, load, `Apply` (§6.3), `Evaluate` (§7.2), save, unlock.
5. Print toasts (§8.1) to stdout.

### 6.1 Parsing LINE (package `internal/parse`)

```go
type Cmd struct {
    Name      string   // basename of the command word, "" if none
    Sudo      bool
    Args      []string // tokens after Name up to the first separator
    Sub       string   // git subcommand, "" if not git
    SubArgs   []string // tokens after Sub
    SudoRetry bool
}
func Parse(line, prev string) Cmd
```

1. `toks := strings.Fields(line)`. Separators `|`, `||`, `&&`, `;`, `&`, `|&` end the first command; only it is analyzed.
2. Skip leading env assignments: tokens matching `^[A-Za-z_][A-Za-z0-9_]*=`.
3. Wrappers, repeatedly:
   - `sudo` or `doas`: `Sudo = true`; skip it, then skip tokens starting with `-`; for `-u -g -C -D -h -p -r -t -U -T` also skip the next token. Remember the index right after `sudo`/`doas` as `afterSudo`.
   - `time`, `nohup`, `command`, `builtin`, `exec`, `nice`, `env`: skip it, then skip tokens starting with `-` and env assignments.
4. `Name = filepath.Base(strings.TrimPrefix(tok, "\\"))`. `Args` = the following tokens until a separator.
5. If `Name == "git"`: walk `Args`; `-C`, `-c`, `--git-dir`, `--work-tree`, `--namespace` skip themselves and the next token; other tokens starting with `-` are skipped; the first other token is `Sub`, and the rest are `SubArgs`.
6. `SudoRetry = Sudo && prev != "" && strings.Join(toks[afterSudo:end], " ") == strings.Join(strings.Fields(prev), " ")`, where `end` is the first separator index (or `len(toks)`). This is what `sudo !!` expands to.

### 6.2 Event (package `internal/stats`)

```go
type Event struct {
    Cmd     parse.Cmd
    Exit    int
    Ms      int64
    Now     time.Time // local time at record
    CwdType string    // "home" | "tmp" | "repo" | "other"
}
```

`CwdType(cwd, home string, tmps []string) string` (first match wins, paths
cleaned; production passes `tmps = [os.TempDir(), "/tmp"]`, deduplicated):
`cwd == home` → `home`; `cwd` equal to or under any entry of `tmps` → `tmp`;
any of `cwd` and up to 64 ancestors contains a `.git` entry (`os.Lstat`) → `repo`;
otherwise `other`. A failing `os.Getwd` gives `other`.

Tracked names (exact list; only these are stored by name):
```
ls ll la exa eza cd pwd cat less more bat head tail grep rg ag find fd fzf
git gh vi vim nvim emacs emacsclient nano code
make cmake ninja cargo rustc go npm npx pnpm yarn bun deno node tsc
python python3 pip pip3 uv ruby gem bundle java javac gradle gradlew mvn
bazel dotnet swift xcodebuild
docker podman kubectl helm terraform ansible ssh scp rsync curl wget
man tldr sudo top htop btop tmux screen clear history echo
mkdir rm mv cp touch chmod chown ln kill pkill killall ps df du tar zip unzip
brew apt dnf pacman jq sqlite3 psql
sl cowsay fortune cmatrix figlet lolcat toilet asciiquarium
gti got gut gerp grpe claer clera mkae amke pyhton pytohn sudp suod vmi nvmi exti eixt
```
(`sudo` appears in `Cmds` only through `Name`, which never is `sudo` after §6.1 step 3; it stays in the list for completeness.)

Sets used below:
- `Typos` = `gti got gut gerp grpe claer clera mkae amke pyhton pytohn sudp suod vmi nvmi exti eixt`
- `Toys` = `sl cowsay fortune cmatrix figlet lolcat toilet asciiquarium`
- `BuildTools` = `make cmake ninja cargo go npm pnpm yarn bun gradle gradlew mvn bazel dotnet swift xcodebuild tsc`

### 6.3 Apply (`func Apply(s *Stats, e Event)`)

`fail := e.Exit != 0 && e.Exit != 130 && e.Exit != 148`.
`gitOK(sub) := Name == "git" && Sub == sub && Exit == 0`.

Counter increments (each +1 when the condition holds), exact keys:

| Key | Condition |
|---|---|
| `commands` | always |
| `failures` | `fail` |
| `not_found` | `Exit == 127` |
| `interrupts` | `Exit == 130` |
| `vim` | Name in `vi vim nvim` |
| `emacs` | Name in `emacs emacsclient` |
| `nano` | Name == `nano` |
| `rm_rf` | Name == `rm`, recursive and force flags (below), `Exit == 0` |
| `gti` | Name == `gti` |
| `typos` | Name in `Typos` |
| `sl` | Name == `sl` |
| `toys` | Name in `Toys` |
| `sudo` | `Sudo` |
| `sudo_retry` | `SudoRetry` |
| `man` | Name == `man` and `Exit == 0` |
| `commits` | `gitOK("commit")` |
| `commits_before_9` | `gitOK("commit")` and `Now.Hour() < 9` |
| `force_push` | `gitOK("push")` and a force flag in `SubArgs` (below) |
| `rebase`, `bisect`, `reflog`, `stash` | Name == `git` and Sub is that word (any exit code) |
| `night` | `Now.Hour()` in 0..3 |
| `early` | `Now.Hour()` in 5..6 |
| `weekend` | `Now.Weekday()` is Saturday or Sunday |
| `long_build` | Name in `BuildTools`, `Ms >= 600000`, `Exit == 0` |
| `marathon` | `Ms >= 3600000` |
| `chmod777` | Name == `chmod`, an arg is `777`, `0777` or `a+rwx`, `Exit == 0` |
| `kill9` | Name in `kill pkill killall`, an arg is `-9`, `-KILL` or `-SIGKILL` |
| `clear` | Name == `clear` |
| `cwd_tmp` | `CwdType == "tmp"` |
| `cwd_repo` | `CwdType == "repo"` |
| `comebacks` | `Exit == 0` and `FailStreak >= 10` (checked before the reset below) |

- rm flags: scan `Args` until `--`. `--recursive` sets r, `--force` sets f. A token starting with a single `-` sets r if it contains `r` or `R`, and f if it contains `f`.
- Force flags: `--force`, `--force-with-lease` (also with `=...`), or a single-dash token containing `f`.

Then, in this order:
1. FailStreak: `fail` → +1; `Exit == 0` → 0; 130 and 148 leave it unchanged.
2. `Cmds[Name]++` if Name is tracked. Set `Counters["distinct"] = len(Cmds)`.
3. Streak, with `day := Now.Format("2006-01-02")`:
   - `day < LastDay` (clock went back): skip this step.
   - `day == LastDay`: nothing.
   - else: `Streak = Streak+1` if `LastDay == Now.AddDate(0,0,-1).Format("2006-01-02")`, otherwise `Streak = 1`. `LastDay = day`. `Year.ActiveDays++`. `Counters["best_streak"] = max(Counters["best_streak"], Streak)`; `Year.BestStreak = max(Year.BestStreak, Streak)`.
4. Year `Now.Format("2006")`: `Commands++`; `Failures++` if `fail`; `Commits++` if `gitOK("commit")`; `LongestMs = max(LongestMs, Ms)`; `Hours[Now.Hour()]++`; `Cmds[Name]++` if tracked; `Cwd[CwdType]++`.

## 7. Achievements

### 7.1 Table (`internal/ach`, one `var All = []Achievement{...}` in this order)

```go
type Achievement struct {
    ID, Title, Desc, Counter string
    Goal int64
}
```

| # | ID | Title | Desc | Counter | Goal |
|---|---|---|---|---|---|
| 1 | first_steps | First Steps | run your first command | commands | 1 |
| 2 | warming_up | Warming Up | run 100 commands | commands | 100 |
| 3 | keyboard_warrior | Keyboard Warrior | run 1,000 commands | commands | 1000 |
| 4 | shell_dweller | Shell Dweller | run 10,000 commands | commands | 10000 |
| 5 | scratchpad | Scratchpad | run 100 commands in a temp dir | cwd_tmp | 100 |
| 6 | escaped_vim | Escaped Vim | exit vim 10 times | vim | 10 |
| 7 | vim_is_home | Vim Is Home | exit vim 500 times | vim | 500 |
| 8 | church_of_emacs | Church of Emacs | open emacs 10 times | emacs | 10 |
| 9 | nano_second | Nano Second | open nano 10 times | nano | 10 |
| 10 | rm_rf_survivor | rm -rf Survivor | run rm -rf and live to tell | rm_rf | 1 |
| 11 | vroom_vroom | Vroom Vroom | type gti 50 times | gti | 50 |
| 12 | fat_fingers | Fat Fingers | make 100 typos | typos | 100 |
| 13 | choo_choo | Choo Choo | run sl | sl | 1 |
| 14 | great_power | With Great Power | use sudo for the first time | sudo | 1 |
| 15 | pretty_please | Pretty Please | retry a command with sudo !! | sudo_retry | 1 |
| 16 | rtfm | RTFM | read a man page | man | 1 |
| 17 | repo_rat | Repo Rat | run 1,000 commands inside git repos | cwd_repo | 1000 |
| 18 | snapshot | Snapshot | make your first git commit | commits | 1 |
| 19 | commit_machine | Commit Machine | make 1,000 git commits | commits | 1000 |
| 20 | before_the_ci | Up Before the CI | make 100 commits before 9am | commits_before_9 | 100 |
| 21 | the_force | May the Force Be With You | force-push a branch | force_push | 1 |
| 22 | rewriting_history | Rewriting History | run git rebase 10 times | rebase | 10 |
| 23 | detective | Detective | start a git bisect | bisect | 1 |
| 24 | time_traveler | Time Traveler | open the git reflog | reflog | 1 |
| 25 | squirrel | Squirrel | run git stash 25 times | stash | 25 |
| 26 | night_owl | Night Owl | run 100 commands between midnight and 4am | night | 100 |
| 27 | early_bird | Early Bird | run 100 commands between 5am and 7am | early | 100 |
| 28 | weekend_warrior | Weekend Warrior | run 500 commands on weekends | weekend | 500 |
| 29 | on_a_roll | On a Roll | use the terminal 10 days in a row | best_streak | 10 |
| 30 | habit_formed | Habit Formed | use the terminal 30 days in a row | best_streak | 30 |
| 31 | patience | Patience of a Saint | wait out a 10-minute build | long_build | 1 |
| 32 | marathon | Marathon | run a command for over an hour | marathon | 1 |
| 33 | command_not_found | Command Not Found | hit "command not found" 100 times | not_found | 100 |
| 34 | abort_mission | Abort Mission | press Ctrl-C 100 times | interrupts | 100 |
| 35 | never_give_up | Never Give Up | succeed right after 10 failures in a row | comebacks | 1 |
| 36 | wide_open | Wide Open | chmod 777 something | chmod777 | 1 |
| 37 | no_mercy | No Mercy | kill -9 a process | kill9 | 1 |
| 38 | easily_amused | Easily Amused | run cowsay, fortune and friends 10 times | toys | 10 |
| 39 | clean_slate | Clean Slate | clear the screen 500 times | clear | 500 |
| 40 | toolbox | Toolbox | use 30 different tracked commands | distinct | 30 |

Table invariants (tested): 40 entries; unique IDs matching `^[a-z0-9_]+$`;
every `Counter` is a key from §6.3 or `best_streak` or `distinct`; `Goal >= 1`;
`len(Title) <= 26` and `len(Desc) <= 44` runes.

### 7.2 Evaluate (`func Evaluate(s *Stats, now time.Time) []Achievement`)

For each achievement in table order: if its ID is not in `Unlocked` and
`Counters[Counter] >= Goal`, set `Unlocked[ID] = now` and append it to the
result. Unlocks are permanent.

## 8. Output

### 8.1 Toasts (`record`)

One line per new unlock, in table order:
```
🏆 Achievement unlocked: <Title> — <Desc>
```
At most 3 lines. If there are more, print 2 lines and then
`🏆 …and <n> more (see term-achievements list)` where `n = total − 2`.

### 8.2 `list`

```
term-achievements · 3/40 unlocked

✓ First Steps                run your first command                       2026-03-10
· Warming Up                 run 100 commands                             [###-------] 37/100
```
Header `term-achievements · <u>/40 unlocked`, then an empty line, then one line
per achievement in table order:
`strings.TrimRight(fmt.Sprintf("%s %-26s %-44s %s", mark, Title, Desc, right), " ")`.
Unlocked: mark `✓`, right = unlock date `2006-01-02` (in the clock's location).
Locked: mark `·`, right = `[` + `#`×k + `-`×(10−k) + `] ` + `cur/goal`, where
`cur = min(counter, Goal)` and `k = cur*10/Goal` (integer division).

### 8.3 `wrapped [--year Y]`

Year defaults to `now.Year()`. If `Years[Y]` is missing or has 0 commands,
print `no commands recorded in <Y> yet` and exit 0.

Card: top border `╭` + 44×`─` + `╮`; each content line `│ ` + content
right-padded with spaces to 42 runes + ` │`; content over 42 runes is cut to 41
runes plus `…`; bottom `╰` + 44×`─` + `╯`. Content lines, in order:

```
term-achievements wrapped · <Y>

<C> commands on <D> days
<F>% failed · best streak <S> days
busiest hour <HH>:00-<HH+1>:00
longest command <dur>
<G> git commits
mostly in: <place> (<P>%)

top commands
  1. git        3,210
  ...
                                  (blank line)
<U> achievements unlocked
  <titles, wrapped>
```

- Numbers use comma thousands separators (`12,345`). Singular forms: `1 command`, `1 day`, `1 git commit`, `1 achievement unlocked`, `best streak 1 day`.
- `F = fmt.Sprintf("%.1f", Failures*100/Commands)`.
- Busiest hour: argmax of `Hours`, lowest hour wins ties; `(h+1)%24`, two digits each (`23:00-00:00`).
- `dur`: `>= 1h` → `<h>h <m>m`; `>= 1m` → `<m>m <s>s`; else `<s>s` (floors; `0s` for 0).
- Place: argmax of `Cwd` with tie order `repo, home, tmp, other`; names `git repos`, `your home dir`, `temp dirs`, `other places`; `P = math.Round(count*100/Commands)`.
- Top commands: `Year.Cmds` sorted by count desc then name asc, at most 5, each `fmt.Sprintf("  %d. %-10s %s", i, name, commas(n))`. None: `  (none tracked yet)`.
- Achievements: those with `Unlocked[ID].Year() == Y`. `U` = their count. Titles in table order; every title except the last gets a trailing `,`. Greedy wrap: the first line is two spaces plus the first title; each next title is appended after one space if the line stays ≤ 42 runes, otherwise it starts a new line of two spaces plus the title. Zero: the `0 achievements unlocked` line and no title lines.

## 9. Shell hooks (embedded `internal/hooks/term-achievements.{zsh,bash,fish}`)

Rules: never print errors, always preserve `$?`/`$status`, never double-register.

zsh (exact):
```zsh
# term-achievements hook for zsh: eval "$(term-achievements init zsh)"
zmodload zsh/datetime 2>/dev/null
typeset -g __termach_cmd="" __termach_prev="" __termach_t0=""
_termach_preexec() {
  __termach_cmd=$1
  __termach_t0=$EPOCHREALTIME
}
_termach_precmd() {
  local e=$?
  if [[ -n $__termach_t0 ]]; then
    local ms=$(( (EPOCHREALTIME - __termach_t0) * 1000 ))
    command term-achievements record --exit "$e" --ms "${ms%.*}" --prev "$__termach_prev" -- "$__termach_cmd" 2>/dev/null
    __termach_prev=$__termach_cmd
    __termach_t0=""
  fi
  return $e
}
autoload -Uz add-zsh-hook
add-zsh-hook preexec _termach_preexec
add-zsh-hook precmd _termach_precmd
```

bash (exact). Bash has no preexec, so the hook reads the newest history entry
with its start time. Commands hidden from history (`HISTCONTROL=ignorespace`,
`ignoredups`, `HISTIGNORE`) are not counted, and durations have 1-second
resolution. Both are documented in the README.
```bash
# term-achievements hook for bash: eval "$(term-achievements init bash)"
__termach_hist=""
__termach_prev=""
_termach_precmd() {
  local e=$? line num ts cmd
  line=$(HISTTIMEFORMAT='%s ' builtin history 1 2>/dev/null)
  read -r num ts cmd <<< "$line"
  num=${num%\*}
  if [[ -n $num && $num != "$__termach_hist" ]]; then
    if [[ -n $__termach_hist ]]; then
      command term-achievements record --exit "$e" --started "$ts" --prev "$__termach_prev" -- "$cmd" 2>/dev/null
      __termach_prev=$cmd
    fi
    __termach_hist=$num
  fi
  return $e
}
case ";${PROMPT_COMMAND:-};" in
  *";_termach_precmd;"*) ;;
  *) PROMPT_COMMAND="_termach_precmd${PROMPT_COMMAND:+;$PROMPT_COMMAND}" ;;
esac
```

fish (exact):
```fish
# term-achievements hook for fish: term-achievements init fish | source
if not functions -q __termach_postexec
    set -g __termach_prev ""
    function __termach_postexec --on-event fish_postexec
        set -l e $status
        set -l d $CMD_DURATION
        test -n "$argv[1]"; or return
        command term-achievements record --exit $e --ms $d --prev "$__termach_prev" -- "$argv[1]" 2>/dev/null
        set -g __termach_prev $argv[1]
    end
end
```

## 10. Package layout

```
term-achievements/
├── main.go                       // os.Exit(cli.Run(os.Args[1:], cli.RealDeps(version)))
├── internal/
│   ├── parse/parse.go            // Cmd, Parse
│   ├── stats/stats.go            // Stats, Year, Event, Apply, CwdType, name sets
│   ├── ach/ach.go                // Achievement, All, Evaluate
│   ├── store/store.go            // Dir, Lock, Load, Save
│   ├── render/{toast.go,list.go,wrapped.go}
│   ├── hooks/hooks.go            // Script(shell) + embedded files
│   └── cli/{cli.go,record.go}
├── .goreleaser.yaml, .github/workflows/{ci.yml,release.yml}
├── README.md
└── LICENSE
```

## 11. Testing requirements

- No test touches the real HOME: state tests set `TERM_ACHIEVEMENTS_HOME` to `t.TempDir()`; CwdType tests build fake dirs under `t.TempDir()`.
- The clock is injected (`Deps.Now`); tests use fixed `time.Date(..., time.UTC)` values. There is no randomness.
- Privacy test: record `git commit -m "SECRET_TOKEN_abc123"`, `mysql -phunter2pass`, a bare `hunter2pass` (not a tracked name), with `--prev "export API_KEY=zzsecretzz"`, from a cwd named `secretproject`; then every file under the state dir must not contain `SECRET_TOKEN_abc123`, `hunter2pass`, `zzsecretzz`, `secretproject`, or `-m`.
- Unit tests for every counter row of §6.3, the streak rules, the table invariants, toast truncation, `list` lines, and the full `wrapped` card.
- Hook tests run real bash (interactive, commands on stdin) and zsh/fish when installed, with a fake `term-achievements` that logs its argv.
- `TestRecordBudget` / `BenchmarkRecord` with a stats file that already holds a busy year (§6).
- CI: `gofmt -l .` empty, `go vet ./...`, `go test ./...` on ubuntu-latest (apt `zsh fish`) and macos-latest.

## 12. Definition of done (v0.1)

- All commands in §4 behave as specified; tests, vet and gofmt are clean; `TestRecordBudget` passes.
- Manual check in a real zsh and bash: `vim` + `:q` ten times shows the Escaped Vim toast.
- README: install, setup per shell, a sample toast, `list` and `wrapped` examples, the privacy section (§5.4), bash limitations (§9), `alias ach=term-achievements` tip, `TERM_ACHIEVEMENTS_DISABLE`, `TERM_ACHIEVEMENTS_HOME`.
- GoReleaser builds linux/darwin × amd64/arm64.

## 13. Later

- Hidden/secret achievements, rarity tiers, points.
- `wrapped --markdown` and an SVG card, `term-achievements reset`, export.
- Proper shell tokenizing (quotes, subshells), counting every command in a pipeline.
- Optional color, PowerShell hook, Homebrew tap.
