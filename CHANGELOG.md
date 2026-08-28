# Changelog

All notable changes to PortLens are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and this project adheres to
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- The port listing, compact summary, verbose report, and JSON now identify the
  **service** behind each port from a curated well-known-port registry (e.g.
  `5432 → PostgreSQL`, `5353 → mDNS`), plus a `SERVICE` column in the listing.
- The listing, summary, verbose report, and JSON now classify each owning
  process as `system` (bundled with the OS, e.g. `kdc`, `mDNSResponder`) or
  `user` (Homebrew, `/Applications`, toolchains) via an `ORIGIN` column /
  `Origin` field. The classification is a heuristic based on the executable
  path and process name; unknown stays blank.
- The listing now has a `PROTOCOL` column (tcp/udp), so a port listening on
  both TCP and UDP (e.g. `88` Kerberos) shows as two clearly-labeled rows
  instead of two look-alike ones. `--filter` matches service and origin too.
- Docker/container awareness: reports and listings show the container that
  owns or publishes a port (name, image, compose project/service), and
  `--kill`/`--restart` target the container instead of its host-side process
  (on macOS this avoids ever signaling the Docker VM). Detection uses the
  local Docker daemon over its unix socket; on Linux the owning process's
  cgroup is used first. Disable with `--no-docker`.
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

- `--json` on a range or multiple ports now emits **only the in-use ports** as
  an array (idle ports are omitted, matching scan mode) and shows the same
  scan progress/ETA/summary on stderr, so stdout stays a pure JSON payload
  ready for `jq` or a file.
- `--log <file>` now works with **every** command (single port, listing, scan,
  JSON, actions) instead of only scan mode: it captures the command's stdout
  to the file. Progress/diagnostics (stderr) are never written, output is
  plain (no color), and interactive mode is disabled. It cannot be combined
  with `--watch`.
- Multi-port inspection (scan mode, JSON) shares one inspection loop, progress
  reporter, and exit-code policy via `scanPorts`, so the commands stay
  consistent and free of duplicated logic.
- Multi-port invocations (ranges, several ports, groups) now use **scan mode**:
  only the ports actually in use are printed, live progress shows a count,
  percent, and ETA (to stderr), and a summary reports how many of the scanned
  ports were found and how long the scan took. Idle ports are no longer an
  error. `--log <file>` writes the full report of every in-use port after the
  scan finishes.
- Port ranges may now span the full port space (1-65535); previously a single
  range was capped at 1024 ports.
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
