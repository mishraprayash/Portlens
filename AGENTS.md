# AGENTS.md

Instructions for AI coding agents working in this repository.

## Mandatory workflow (follow every session, no exceptions)

This repo is open source. All work on features, fixes, and refactors MUST
follow [CONTRIBUTING.md](CONTRIBUTING.md) and be gated by the same checks CI
runs. Before opening a PR or pushing to `main`:

1. Read `CONTRIBUTING.md` first (principles, layout, conventions, PR process).
2. Follow the house conventions:
   - Conventional commits (`feat:`, `fix:`, `test:`, `docs:`, `chore:`,
     `refactor:`) matching the existing git history.
   - `gofmt`-clean Go; doc comments on exported identifiers.
   - Platform code only inside `internal/platform` build-tagged files.
   - Facts vs. inferences model from `docs/detection.md`.
   - Local-first: never add telemetry or transmit process/environment data.
   - No secrets, ever.
3. Run the full local gate before committing:
   ```bash
   make check        # gofmt check + go vet + full test suite (-count=1)
   ```
   CI runs the same checks on Linux and macOS; if CI fails, fix it before
   pushing further.
4. For every change:
   - Add/update tests covering the change.
   - Update `CHANGELOG.md` under `[Unreleased]`.
   - Update user-facing docs/help text (`README.md`, `docs/usage.md`,
     `cmd/usage.go`) if behavior changed.
5. Commit focused changes and push, then confirm CI is green.

## Environment notes

- Build/test with `CGO_ENABLED=0` (set by the Makefile automatically; required
  for static binaries and to avoid a macOS linker issue).
- Remote is HTTPS (`https://github.com/mishraprayash/Portlens.git`), branch
  `main`. Pushing `.github/workflows/` requires a token with `workflow` scope.
- Go 1.23+.
