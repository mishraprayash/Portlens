# Contributing to PortLens

Thanks for your interest in contributing. PortLens is a small, local-first
developer utility, and we want to keep it fast, correct, and safe.

## Principles

1. **Local-first, always.** Never add telemetry, cloud calls, or anything that
   transmits process/environment data.
2. **Never silently kill.** Destructive actions must confirm first.
3. **Separate facts from inferences.** Anything guessed must be labeled as such.
4. **Isolate platform behavior.** All OS-specific code belongs behind the
   interfaces in `internal/platform` and in build-tagged `darwin_*.go` /
   `linux_*.go` files. Never scatter shell commands through the codebase.

## Getting started

```bash
make build          # build
make test           # run tests (CGO_ENABLED=0 is set automatically)
make vet fmt        # static checks
```

## Adding a platform provider

To add a new OS (e.g. Windows), create `internal/platform/windows_*.go` files
implementing the existing interfaces (`PortResolver`, `NetworkInspector`,
`ProcessInspector`, `ProcessTreeProvider`, `ClipboardProvider`,
`ProcessController`) and wire them into `internal/platform/new.go`. No other
code needs to change.

## Testing

- Unit tests live next to the code they test.
- Integration tests in `tests/integration` spawn controlled test processes —
  never assume a particular process is present on the developer's machine.
- Run the full suite with `make test` before opening a PR.

## Style

- `gofmt`-formatted Go.
- Meaningful comments on exported types and functions.
- Keep dependencies minimal; prefer the standard library and native system
  APIs over pulling in large frameworks.

## Pull requests

1. Open an issue describing the bug or feature first.
2. Keep changes focused and include tests.
3. Run `make test && make vet` and ensure `gofmt -l` is clean.
4. Update `CHANGELOG.md` under the `Unreleased` section.

By contributing you agree to license your work under the MIT License.
