# term-achievements

Xbox-style achievements for your shell. A tiny hook reports each finished command, and you get a toast before your next prompt:

```
🏆 Achievement unlocked: Escaped Vim — exit vim 10 times
```

## Quickstart

```sh
go install github.com/dpologdvinity/term-achievements@latest
echo 'eval "$(term-achievements init zsh)"' >> ~/.zshrc   # or bash / fish, see Setup
exec zsh                                                    # restart the shell
term-achievements list                                      # see your progress
```

## How it works

A small shell hook runs `term-achievements record` after every command. `record` parses the
command line in memory, updates a handful of counters in one JSON file, unlocks any
achievement whose counter reached its goal, and prints a one-line toast. `list` and
`wrapped` only read that file. The code is split into small packages (`parse`, `stats`,
`ach`, `store`, `render`, `hooks`, `cli`); see [ARCHITECTURE.md](ARCHITECTURE.md) for the
data flow and design decisions.

## Contents

[Quickstart](#quickstart) · [How it works](#how-it-works) · [Install](#install) · [Setup](#setup) · [Commands](#commands) · [Privacy](#privacy) · [Bash notes](#bash-notes) · [Performance](#performance) · [Environment](#environment) · [Achievements](#achievements) · [Development](#development) · [License](#license)

## Install

```sh
brew install dpologdvinity/tap/term-achievements   # macOS and Linux
# or
go install github.com/dpologdvinity/term-achievements@latest
```

Or download an archive (linux/darwin, amd64/arm64) from the GitHub releases page.

## Setup

Add it at the end of your rc file.

- zsh: add `eval "$(term-achievements init zsh)"` to `~/.zshrc`
- bash: add `eval "$(term-achievements init bash)"` to `~/.bashrc`
- fish: add `term-achievements init fish | source` to `~/.config/fish/config.fish`

Tip: `alias ach=term-achievements` (not shipped; add it yourself).

## Commands

```
term-achievements list                 show achievements and progress
term-achievements wrapped [--year Y]   your year in the terminal
term-achievements init <shell>         print the shell hook (zsh, bash, fish)
term-achievements --version            print the version
```

`list` (excerpt):

```
term-achievements · 3/40 unlocked

✓ First Steps                run your first command                       2026-10-23
· Warming Up                 run 100 commands                             [#---------] 14/100
· Keyboard Warrior           run 1,000 commands                           [----------] 14/1000
· Shell Dweller              run 10,000 commands                          [----------] 14/10000
· Scratchpad                 run 100 commands in a temp dir               [----------] 0/100
✓ Escaped Vim                exit vim 10 times                            2026-10-23
· Vim Is Home                exit vim 500 times                           [----------] 12/500
```

`wrapped`:

```
╭────────────────────────────────────────────╮
│ term-achievements wrapped · 2026           │
│                                            │
│ 14 commands on 1 day                       │
│ 0.0% failed · best streak 1 day            │
│ busiest hour 22:00-23:00                   │
│ longest command 0s                         │
│ 1 git commit                               │
│ mostly in: git repos (100%)                │
│                                            │
│ top commands                               │
│   1. vim        12                         │
│   2. git        1                          │
│   3. gti        1                          │
│                                            │
│ 3 achievements unlocked                    │
│   First Steps, Escaped Vim, Snapshot       │
╰────────────────────────────────────────────╯
```

## Privacy

Only counters and aggregates are stored. The command line, the previous command line, every argument and the working directory path are used in memory during one `record` run and never written anywhere. A command name is stored only if it is on a fixed list of well-known tools (for example `git`, `vim`, `make`); anything else, such as a password typed at the prompt by mistake, only bumps generic counters. Counter keys are a fixed set and are never built from your input. There is no network access.

## Bash notes

Bash has no preexec hook, so the bash hook reads the newest history entry:

- Commands hidden from history (`HISTCONTROL=ignorespace`, `ignoredups`, `HISTIGNORE`) are not counted.
- Durations have 1-second resolution.

## Performance

`record` does its own work (lock, load, update, save) in well under 10 ms, and has no network or subprocess. Process start-up of the binary adds to that and depends on your machine. The stats file is renamed into place but not `fsync`ed, because a sync can cost tens of milliseconds on some disks; after a power loss mid-save, stats can be reset.

## Environment

- `TERM_ACHIEVEMENTS_HOME` — state directory (default `$XDG_STATE_HOME/term-achievements`, else `~/.local/state/term-achievements`)
- `TERM_ACHIEVEMENTS_DISABLE` — set to any non-empty value to stop recording

## Achievements

- **First Steps** — run your first command
- **Warming Up** — run 100 commands
- **Keyboard Warrior** — run 1,000 commands
- **Shell Dweller** — run 10,000 commands
- **Scratchpad** — run 100 commands in a temp dir
- **Escaped Vim** — exit vim 10 times
- **Vim Is Home** — exit vim 500 times
- **Church of Emacs** — open emacs 10 times
- **Nano Second** — open nano 10 times
- **rm -rf Survivor** — run rm -rf and live to tell
- **Vroom Vroom** — type gti 50 times
- **Fat Fingers** — make 100 typos
- **Choo Choo** — run sl
- **With Great Power** — use sudo for the first time
- **Pretty Please** — retry a command with sudo !!
- **RTFM** — read a man page
- **Repo Rat** — run 1,000 commands inside git repos
- **Snapshot** — make your first git commit
- **Commit Machine** — make 1,000 git commits
- **Up Before the CI** — make 100 commits before 9am
- **May the Force Be With You** — force-push a branch
- **Rewriting History** — run git rebase 10 times
- **Detective** — start a git bisect
- **Time Traveler** — open the git reflog
- **Squirrel** — run git stash 25 times
- **Night Owl** — run 100 commands between midnight and 4am
- **Early Bird** — run 100 commands between 5am and 7am
- **Weekend Warrior** — run 500 commands on weekends
- **On a Roll** — use the terminal 10 days in a row
- **Habit Formed** — use the terminal 30 days in a row
- **Patience of a Saint** — wait out a 10-minute build
- **Marathon** — run a command for over an hour
- **Command Not Found** — hit "command not found" 100 times
- **Abort Mission** — press Ctrl-C 100 times
- **Never Give Up** — succeed right after 10 failures in a row
- **Wide Open** — chmod 777 something
- **No Mercy** — kill -9 a process
- **Easily Amused** — run cowsay, fortune and friends 10 times
- **Clean Slate** — clear the screen 500 times
- **Toolbox** — use 30 different tracked commands

## Development

Requires Go 1.26 or newer. There are no third-party dependencies.

```sh
go build ./...                 # build
go test ./...                  # run the tests (add -short to skip the speed test)
go vet ./... && gofmt -l .     # static checks; gofmt prints nothing when clean
golangci-lint run ./...        # linters (config in .golangci.yml)
go test -bench Record -run '^$' ./internal/cli/   # time one `record` call
```

The shell hook tests run real shells: bash always, zsh and fish when installed.
For how the code is organized and why, see [ARCHITECTURE.md](ARCHITECTURE.md); for the contribution workflow, see [CONTRIBUTING.md](CONTRIBUTING.md). The exact
behavior is specified in [docs/SPEC.md](docs/SPEC.md).

## License

MIT
