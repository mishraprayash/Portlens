# Testing

How PortLens is tested, how to run the suite, and the platform caveats you will
hit as a contributor.

## Quick start

```bash
make check     # the gate: gofmt + vet + build + test + cross-compile
make test      # unit + integration tests only (no cache, -count=1)
make cover     # same suite, prints an aggregate coverage percentage
make bench     # performance regression suite (see docs/performance.md)
```

## The one rule: always go through `make`

The `Makefile` exports `CGO_ENABLED=0`. A bare `go test ./...` from your editor
or shell uses the toolchain default (`CGO_ENABLED=1`), and on **macOS + Go
1.23.x** that produces test binaries dyld refuses to load:

```
dyld[…]: missing LC_UUID load command
signal: abort trap
FAIL    github.com/mishraprayash/Portlens/cmd
```

This is a toolchain issue, not a code issue. Workarounds, in order of
preference:

1. Use `make test` / `make check` (recommended).
2. If you must run `go test` directly, prefix it:
   `CGO_ENABLED=0 go test ./...`

CI sets `CGO_ENABLED: "0"` globally, so it never hits this.

## What lives where

| Kind | Location | Rules |
|------|----------|-------|
| Unit tests | Next to the code (`package_test.go`), same package | Prefer pure logic with no OS involvement: parsers, matchers, config round-trips, render output |
| Table-driven tests | House style for flags/parsing | See `cmd/root_test.go` |
| Render assertions | `internal/render/*_test.go` | Assert the important lines appear, not the whole frame |
| Benchmarks | `*_bench_test.go` | Wired to `make bench` and `docs/performance.md` |
| Integration | `tests/integration/` | Spawn **controlled** test processes (an HTTP-server helper re-invoking the test binary). Never assume a particular process is running on the machine |

Never write tests that depend on the author's environment: specific ports in
use, specific PIDs, or processes that happen to be running.

## Coverage

```bash
make cover     # writes coverage.out, prints the aggregate percentage
```

`coverage.out` is gitignored (`*.out`). CI runs `make cover` on Linux and
posts the total to the job summary. When adding behavior, add tests that move
the package's number — the weakest packages today are `internal/inspector`,
`internal/actions`, and the exit-code mapping in `internal/model`.

Known gaps worth attacking first:

- `model.MapExitCode` — the documented exit-code contract (docs/exit-codes.md)
- The interactive and watch loops in `cmd/` (`runInteractive`, `runWatch`)
- Config disk I/O (`internal/config.Load` / `Save`)

## Race detector

```bash
make race      # Linux only
```

`-race` requires cgo, which is blocked on macOS by the same LC_UUID toolchain
issue described above. CI runs the race job on `ubuntu-latest` only. Run
`make race` in a Linux container or rely on CI if you are on macOS.

## Cross-compilation

`make check` ends with a cross-compile matrix (darwin/arm64, darwin/amd64,
linux/amd64, linux/arm64). Because PortLens has no cgo, this catches
build-tag and platform-isolation mistakes cheaply — if you add OS-specific
code, this is where an accidental import of a darwin-only symbol on linux
shows up before CI does.

Adding a new platform? See [CONTRIBUTING.md](../CONTRIBUTING.md) §"How to add
a platform" — the matrix is where you prove it builds.
