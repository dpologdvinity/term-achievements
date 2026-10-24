# Architecture

This document explains how term-achievements is built and why. For usage, see
[README.md](README.md). For exact behavior (counter rules, output formats), see
[docs/SPEC.md](docs/SPEC.md), which is the source of truth.

## Goals that shape the design

1. **Fast.** `record` runs before every prompt. It must finish in under 10 ms, with no
   network, no subprocess and no git call.
2. **Private.** Command lines, arguments and directory paths are never written to disk.
   Only counters and aggregates are stored.
3. **Simple.** One static binary, no third-party dependencies, one JSON file of state.
4. **Safe.** `record` never prints an error and always exits 0, so a bug here can never
   break a user's shell prompt.

## Data flow

```
 shell prompt
     │  hook (zsh / bash / fish) runs after each command
     ▼
 term-achievements record --exit N --ms N --prev LINE -- LINE
     │
     ├─ parse.Parse(line, prev)          command line  →  parse.Cmd (in memory only)
     ├─ stats.CwdType(cwd, home, tmps)   directory     →  "home" | "tmp" | "repo" | "other"
     ├─ store.Lock (exclusive, 5 ms)     one writer at a time
     ├─ store.Load                       stats.json     →  *stats.Stats
     ├─ stats.Apply(s, event)            update counters, streaks, yearly aggregates
     ├─ ach.Evaluate(s, now)             unlock achievements whose goal is reached
     ├─ store.Save                       *stats.Stats   →  stats.json (atomic rename)
     └─ render.Toasts(unlocked)          print 🏆 lines to stdout

 term-achievements list | wrapped
     └─ store.Lock (shared) → store.Load → render.List / render.Wrapped
```

## Packages

Dependencies point downward; there are no cycles.

```
main ──► cli ──► render ──► ach ──► stats ──► parse
          │                          ▲
          ├──► store ────────────────┘
          └──► hooks
```

| Package | Responsibility |
|---|---|
| `main` | Entry point. Holds the `version` variable (set by GoReleaser) and calls `cli.Run`. |
| `internal/cli` | Argument dispatch for `list`, `wrapped`, `init`, `record`, `--help`, `--version`. All side effects (clock, environment, working directory, output streams) come in through the `Deps` struct, so tests inject fakes. `record.go` is the hot path. |
| `internal/parse` | Turns a command line into a `Cmd`: command name, `sudo`, git subcommand, sudo-retry. Splits on whitespace only; quoting is not interpreted. The line lives in memory for one run. |
| `internal/stats` | The `Stats` data model and `Apply`, which turns one finished command into counter updates. Also `CwdType` and the fixed set of tracked command names. |
| `internal/ach` | The declarative table of 40 achievements (`All`) and `Evaluate`, which unlocks the ones whose counter has reached its goal. |
| `internal/store` | Locked, atomic reading and writing of `stats.json` in the state directory. |
| `internal/render` | Pure functions from data to text: toasts, the `list` table and the `wrapped` card. |
| `internal/hooks` | The zsh, bash and fish hook scripts, embedded into the binary with `go:embed`. |

## Key decisions

### Counters, not logs
Every achievement is "a counter reaches a goal". `Apply` maps an event to increments of
a *fixed* set of counter keys (`stats.CounterKeys`). Because no key is ever built from
user input, a typo or a pasted password cannot create a new entry. The achievement
table is data: adding an achievement means adding one row to `ach.All`, plus a counter
in `Apply` if none fits.

### Tracked command names
Per-command counts (`Cmds`, and the "top commands" in `wrapped`) are kept only for a
fixed list of well-known tools (`stats.Tracked`). Any other command name, including a
secret typed at the prompt by mistake, only raises generic counters such as `commands`.
`TestRecordPrivacy` and `TestTrackedOnly` guard this.

### State file and locking
State is one JSON file (`stats.json`) plus a lock file (`stats.lock`) in the state
directory (`$TERM_ACHIEVEMENTS_HOME`, else `$XDG_STATE_HOME/term-achievements`, else
`~/.local/state/term-achievements`).

- **Writers** (`record`) take an exclusive `flock`, retrying without blocking every
  500 µs for at most 5 ms. If another shell holds the lock, the event is dropped. Losing
  one event is better than delaying a prompt.
- **Readers** (`list`, `wrapped`) take a shared lock with a 2 s budget and report
  "busy" if they cannot get it.
- **Saves** write `stats.json.tmp-<pid>` and `rename` it over `stats.json`, so a reader
  never sees a half-written file. The file is deliberately *not* `fsync`ed: a sync
  measured about 30 ms on some disks, which would break the 10 ms budget. The cost is
  that a power loss during a save can reset the stats.
- **Corrupt files** are moved to `stats.json.corrupt-<unix time>` and the program starts
  empty. `record` stays silent about this; `list` and `wrapped` print one warning.
- **Newer versions**: a file with `version` greater than 1 is never overwritten. `record`
  does nothing and `list` asks the user to upgrade.

### Time and the clock
`Apply` takes `Event.Now` from the caller; nothing in `stats`, `ach` or `render` reads
the real clock. Tests use fixed times. The daily streak compares local calendar days as
strings. If the clock goes backwards, the streak step is skipped, so the streak neither
resets nor counts twice.

### Hooks
The hooks are small shell scripts with three rules: never print errors, always preserve
the exit status of the user's command, and never register twice.

- **zsh** uses `preexec`/`precmd` and `$EPOCHREALTIME` for millisecond durations.
- **fish** uses the `fish_postexec` event and `$CMD_DURATION`.
- **bash** has no `preexec`, so the hook (run from `PROMPT_COMMAND`) reads the newest
  history entry and its timestamp. Commands hidden from history are not counted, and
  durations have one-second resolution.

### Why `record` never fails
`runRecord` catches panics and turns every error (bad flags, unreadable state, busy
lock, full disk) into a silent exit 0. A broken tool must not break the user's prompt.

## Testing strategy

- **Unit tests** per package with the standard `testing` package only.
- **No real HOME.** State tests set `TERM_ACHIEVEMENTS_HOME` to `t.TempDir()`.
- **Injected clock.** `cli.Deps.Now` returns a fixed time; there is no randomness.
- **Hook tests** run the real shells (bash always, zsh and fish when installed) with a
  fake `term-achievements` that logs its arguments.
- **Privacy test.** Records lines containing secrets, then asserts that no file in the
  state directory contains them.
- **Speed.** `TestRecordBudget` (median of 20 in-process runs under 10 ms) and
  `BenchmarkRecord`. Process start-up of the binary is not included.
- **CI** (`.github/workflows/ci.yml`) runs gofmt, `go vet` and `go test -race` on Linux
  and macOS, `govulncheck`, and a GoReleaser snapshot build.

## Extending

- **New achievement:** add a row to `ach.All`. If no existing counter fits, add the key
  to `stats.CounterKeys`, bump it in `stats.Apply`, and add a case to
  `TestCounters`. `TestTableInvariants` checks the table automatically.
- **New tracked command:** add the name to `stats.Tracked`.
- **New shell:** add a hook script in `internal/hooks`, extend `hooks.Script` and the
  `go:embed` line, and add a test that runs the shell.
- **Schema change:** bump `Stats.Version` and migrate in `store.Load`; older binaries
  refuse newer files (`store.ErrNewer`) instead of corrupting them.

## Release

Pushing a tag like `v0.1.0` runs `.github/workflows/release.yml`, which uses GoReleaser
(`.goreleaser.yaml`) to build static (`CGO_ENABLED=0`) binaries for linux and darwin on
amd64 and arm64, and publishes archives and `checksums.txt` to the GitHub release.
