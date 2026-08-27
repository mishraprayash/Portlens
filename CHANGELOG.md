# Changelog

All notable changes to PortLens are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and this project adheres to
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Multiple ports, port ranges (`3000-3010`), and `--all` in a single invocation;
  `--json` emits a JSON array when more than one port is inspected.
- Inverse lookup with `--pid <pid>` (includes descendants) and
  `--name <query>` (case-insensitive substring, or `/regex/`) so you can find
  and act on ports starting from a process.
- `--watch` (with `--interval`) live-rendering of a port or the full listing.
- `--notify` desktop notifications (macOS via `osascript`, Linux via
  `notify-send`) when a watched port goes up, goes down, or changes owner.
- Named port groups via `portlens config add|list|show|remove|path`, usable as
  `portlens @<group>`, stored in a local JSON config file.

### Changed

- `portlens <port>` now shows a compact summary by default; use `--verbose`
  (or `-v`) for the full detailed report. `-v` is no longer a `--version`
  alias (`--version` remains).

## [0.1.0] - 2026-08-27

Initial release.

### Added

- `portlens <port>` inspection with process, project, exposure, process tree,
  network, and interpretation sections.
- `portlens` (no argument) listing of listening ports with `--sort`, `--filter`,
  and `--tcp`.
- `--tree`, `--connections`, `--json`, `--kill`, `--kill --force`, `--restart`,
  `--open`, and `--history` commands.
- OS abstraction layer with macOS (lsof) and Linux (/proc) providers, plus a
  cgo-free build for trivial cross-compilation.
- Runtime and framework detection for Node.js (NestJS, Next.js, Express,
  Fastify, Prisma, etc.), Python, Go, Java, Rust, Docker, and common databases.
- Local SQLite-backed port history (stored locally, never transmitted).
- Cautious exposure/risk assessment (LOW RISK / WARNING / POTENTIALLY DANGEROUS).
- Safe process management: graceful SIGTERM first, explicit `--force` for
  SIGKILL, confirmation prompts, and no privilege escalation.
- Interactive single-key terminal UI with a plain-text fallback.
- Documented JSON schema and exit codes.
- Unit and integration tests (including controlled test processes).
