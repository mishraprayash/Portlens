# Contributing to PortLens

Thanks for your interest in contributing. PortLens is a small, local-first
developer utility, and we want to keep it fast, correct, safe, and easy for
new contributors to reason about.

Please also read our [Code of Conduct](CODE_OF_CONDUCT.md) and our
[Security policy](SECURITY.md).

## Table of contents

1. [Project principles](#project-principles)
2. [Before you start](#before-you-start)
3. [Environment setup](#environment-setup)
4. [Project layout](#project-layout)
5. [Development workflow](#development-workflow)
6. [Running and testing](#running-and-testing)
7. [Code conventions](#code-conventions)
8. [How to add a feature](#how-to-add-a-feature)
9. [How to add a platform (e.g. Windows)](#how-to-add-a-platform-eg-windows)
10. [Commits and pull requests](#commits-and-pull-requests)
11. [Getting help](#getting-help)

## Project principles

These are non-negotiable. If a change fights one of these, it will be rejected
or reworked:

1. **Local-first, always.** Never add telemetry, cloud calls, or anything that
   transmits process/environment data. PortLens reads the machine it runs on
   and keeps everything (history, config) on that machine.
2. **Never silently kill.** Destructive actions must confirm first. Graceful
   (SIGTERM) before force (SIGKILL), and never escalate privileges.
3. **Separate facts from inferences.** Anything guessed must be labeled as
   such. See `docs/detection.md` for the Facts vs. Inferences model.
4. **Isolate platform behavior.** All OS-specific code belongs behind the
   interfaces in `internal/platform`, in build-tagged `*_darwin.go` /
   `*_linux.go` files. No shell commands or OS calls anywhere else.
5. **Keep it lean.** Prefer the standard library and native system APIs over
   large frameworks. New dependencies need a strong justification.

## Before you start

- Open an issue describing the bug or feature first, so work isn't duplicated
  and design gets a quick sanity check.
- Small, focused changes are much easier to review than big rewrites.
- If you are touching the CLI, run `portlens --help` after your change and keep
  the help text accurate.

## Environment setup

Requirements:

- Go 1.23+ (`go.mod` declares `go 1.23.0`)
- `make` (optional but recommended)
- No cgo is required — PortLens builds with `CGO_ENABLED=0`.

```bash
git clone https://github.com/mishraprayash/Portlens.git
cd Portlens
make build            # builds ./bin/portlens
```

The `Makefile` exports `CGO_ENABLED=0` automatically. Keep it that way: it
produces static binaries, makes cross-compilation trivial, and sidesteps a
linker incompatibility between older Go toolchains and recent macOS releases.

## Project layout

```
cmd/                 CLI layer: flag parsing, dispatch, watch mode, config subcommand
internal/            Non-exported implementation packages
  model/             Shared data types (Report, Listener, ProcessInfo, ...)
  platform/          OS abstraction: interfaces + darwin/linux providers
  inspector/         Orchestrates platform providers into a Report; process search
  detect/            Project/runtime/framework detection heuristics
  service/           Application service facade coordinating domain use cases
  render/            Human output (summary, report, tree, connections, JSON)
  actions/           State-changing operations (kill, restart, open, copy)
  config/            User config: named port groups (@name)
  exitcode/          Process exit codes
  version/           Build version
tests/integration/   End-to-end tests that spawn controlled test processes
docs/                Design and user documentation
```

See [docs/architecture.md](docs/architecture.md) for the layered design in
detail. The rule of thumb: **dependencies point inward**. `cmd` may use any
`internal` package; `internal/*` never imports `cmd`; platform code never leaks
past `internal/platform`.

## Development workflow

```bash
make fmt             # gofmt all files
make lint            # gofmt check (fails) + go vet
make test            # full unit + integration suite (no cache)
make check           # lint + test: the same gate CI runs
make build           # build to ./bin/portlens
make install         # install to $GOBIN
make cross           # verify macOS/Linux cross-compilation
```

To try the binary while developing:

```bash
make build
./bin/portlens              # list listening ports
./bin/portlens 3000         # compact summary
./bin/portlens 3000 -v      # full report
./bin/portlens 3000 --watch # live refresh
```

## Running and testing

- **Unit tests** live next to the code they test (`package_test.go`), in the
  same package. Test pure logic with no OS involvement whenever possible
  (parsers, matchers, config round-trips, render output).
- **Integration tests** live in `tests/integration` and spawn **controlled**
  test processes (an HTTP server helper). Never assume a particular process is
  running on the developer's machine.
- Run everything with `make test` before opening a PR. CI runs the same
  commands with `-count=1` on both Linux and macOS.

When writing tests:

- Cover both success and failure paths.
- For flags/parsing, table-driven tests are the house style (see
  `cmd/root_test.go`).
- New render output should get a render test asserting the important lines
  appear (see `internal/render/summary_test.go`).
- Never write tests that depend on the author's environment (ports, PIDs, or
  processes that happen to be running).

## Code conventions

- **Formatting:** always `gofmt`-clean. `make lint` enforces this.
- **Comments:** exported identifiers need doc comments. Use comments to explain
  *why*, not restate *what*.
- **Errors:** wrap errors with context (`fmt.Errorf("...: %w", err)`). Surface
  user-facing errors through the existing exit-code model in
  `internal/exitcode` (documented in `docs/exit-codes.md`).
- **Facts vs. inferences:** when your code guesses, add to `Inferences` or
  `Facts` on the `model.Report`, never hard-code a claim as certain.
- **Flags:** add flags in `cmd/root.go` with both long and (when sensible)
  short forms, keep them documented in `cmd/usage.go`, and validate
  combinations in `parseArgs` (see existing `--force`/`--kill` checks).
- **No shelling out** except inside `internal/platform` build-tagged files.
- **No secrets, ever.** The user configuration is user-local; keep it that way
  and never log its contents.

## How to add a feature

A typical end-to-end feature touches:

1. **`internal/model`** — add any new data types or fields.
2. **`internal/platform`** — OS-level gathering (only if you need new OS data).
3. **`internal/inspector`** — orchestration/search logic that assembles a
   `Report` or `PortEntry`.
4. **`internal/render`** — a renderer for the new output.
5. **`cmd/root.go`** — the flag(s) and wiring in `Execute`.
6. **`cmd/usage.go`** and `README.md` / `docs/usage.md` — help text and docs.
7. **`CHANGELOG.md`** — a bullet under `[Unreleased]`.

Write tests as you go; don't defer them to the end.

## How to add a platform (e.g. Windows)

To support a new OS, create `internal/platform/windows_*.go` files that
implement the existing interfaces — `PortResolver`, `NetworkInspector`,
`ProcessInspector`, `ProcessTreeProvider`, `ClipboardProvider`,
`ProcessController` — plus the small standalone functions (`Notify`, `OpenURL`).
Wire the constructors into `internal/platform/new.go`. No other code needs to
change. Keep every platform command inside these files.

## Commits and pull requests

1. **Branch:** work on a descriptive branch (`fix/kill-race`,
   `feat/port-search`), not directly on `main`.
2. **Commit style:** conventional commits, matching the existing history
   (`feat:`, `fix:`, `test:`, `docs:`, `chore:`, `refactor:`). Write a concise
   subject line and, when useful, a body explaining the *why*.
3. **Focused diffs:** one logical change per PR. Include tests and docs with
   the change.
4. **Run the gate before pushing:**
   ```bash
   make check
   ```
   This runs `gofmt` check, `go vet`, and the full test suite. CI runs the same
   thing on Linux and macOS and must pass.
5. **Update `CHANGELOG.md`** under `[Unreleased]`, and `README.md`/docs if the
   user-facing behavior changed.
6. **Review:** keep PRs reviewable — add a short description, reference the
   issue (`Fixes #123`), and respond to review comments.

By contributing you agree to license your work under the MIT License (see
[LICENSE](LICENSE)).

## Getting help

- Open a discussion or issue on GitHub for design questions.
- Read `docs/` first — `architecture.md`, `usage.md`, `detection.md`, and
  `security.md` answer most "how does this work?" questions.
- For security-related reports, follow [SECURITY.md](SECURITY.md) and do not
  open a public issue.
