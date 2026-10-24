# Contributing

Thanks for helping with term-achievements! This guide covers the local workflow and the
conventions for pull requests. For how the code is organized, read
[ARCHITECTURE.md](ARCHITECTURE.md); for exact behavior, read [docs/SPEC.md](docs/SPEC.md).

## Setup

You need Go 1.26 or newer. There are no third-party dependencies to install.

```sh
git clone https://github.com/dpologdvinity/term-achievements.git
cd term-achievements
go build ./...
```

Optional but recommended: `golangci-lint` (v2), and `zsh` and `fish` so the hook tests run
against real shells instead of skipping.

## Everyday commands

```sh
go test ./...                                                                 # all tests (add -short to skip the speed test, -race for the race detector)
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out  # overall statement coverage (CI requires at least 85%)
go vet ./...                                                                  # static checks
gofmt -l .                     # must print nothing; fix with `gofmt -w .`
golangci-lint run ./...        # linters, configured in .golangci.yml
go test -bench Record -run '^$' ./internal/cli/   # time one `record` call
```

CI runs the same checks (`.github/workflows/ci.yml`): dependency verification, lint,
tests with `-race` and a coverage gate on Linux and macOS, a vulnerability scan,
cross-compilation for all release targets, and a GoReleaser dry run.

## Rules of the road

- **Tests first.** New behavior needs a test. For a bug, write a failing test, then fix it.
- **Privacy is a hard rule.** Never store a command line, an argument or a path. Only fixed
  counter keys and names from `stats.Tracked` may reach the disk (see `TestRecordPrivacy`).
- **`record` must stay fast and silent.** It has a 10 ms budget, no network, no subprocess,
  never writes to stderr and always exits 0.
- **No new dependencies** without a strong reason; the standard library is enough so far.
- **Document your code.** Every package, exported symbol and test gets a short, clear doc
  comment. Comment the *why*, not the obvious *what*.
- **Spec first for behavior changes.** If you change what the tool does, update
  `docs/SPEC.md` and the README in the same pull request.

## Commits and pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org/): `type(scope): description`,
  for example `fix(parse): skip the value after nice -n`. Common types: `feat`, `fix`,
  `docs`, `test`, `refactor`, `chore`, `ci`.
- Keep commits small and focused: one logical change each.
- Branch from `master`, open a pull request into `master`, and wait for CI to pass.
  PRs are squash-merged, so the PR title should be a Conventional Commit message too.
- Never commit secrets, `.env` files, or tool-specific files such as `CLAUDE.md`, `.claude/`
  or `AGENTS.md`.

## Adding things

See "Extending" in [ARCHITECTURE.md](ARCHITECTURE.md) for how to add an achievement, a
tracked command or a shell.

## Releasing (maintainers)

Push a tag such as `v0.2.0` on `master`. The release workflow runs the tests and then
GoReleaser publishes the archives and `checksums.txt` to a GitHub release.
