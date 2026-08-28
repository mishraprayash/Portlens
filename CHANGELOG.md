# Changelog

All notable changes to PortLens are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and this project adheres to
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed

- **`--restart` now gracefully terminates the existing process first**: Shuts down the running process tree and waits for the port to release before relaunching, eliminating `EADDRINUSE` socket conflicts. Stdio is detached into `/dev/null` so background logs do not corrupt the terminal session.
- **`LaunchProcess` directly launched by shell**: Fixed detection so that when a process is the immediate child of an interactive shell, PortLens restarts the process itself rather than mistakenly trying to re-execute the parent shell.
- **Subcommand routing with global flags**: `portlens [flags] config ...` (e.g. `portlens --no-color config list`) now dispatches to the config subcommand instead of erroring with an invalid port.
- **Git worktrees, submodules, and branch paths**: Supports `.git` files with `gitdir:` indirection, preserves full multi-segment branch names (e.g. `feature/auth/oauth2`), parses SSH remote URLs, and handles detached HEAD states.
- **Interactive TUI escape sequence drainage & PID verification**: Drains trailing bytes of ANSI escape sequences (e.g. arrow keys) so they do not spill into the shell upon exit, and confirms the target PID is still alive before signaling to prevent killing recycled PIDs.
- **Watch mode immediate signal cancellation**: Migrated `runWatch` to `signal.NotifyContext` so `Ctrl-C` or `SIGTERM` cancels in-flight inspection passes immediately instead of blocking until the current tick finishes.
- **Unbounded `lsof` execution timeout**: Added a 10-second safety deadline to macOS `runLsof` when invoked with an unbounded context, preventing indefinite blocking on unresponsive network mounts (NFS/SMB).
- **Linux `/proc` socket link slice bounds safety**: Added defensive validation for socket symlinks in `/proc/<pid>/fd`, eliminating potential slice out-of-bounds panics on malformed symlinks.
- **Non-blocking browser launching on Linux**: Launched browser processes asynchronously via `cmd.Start()`, preventing foreground browser processes from locking the PortLens CLI.

### Changed

- **Multi-port and range scan bulk pre-filtering**: Scans (`portlens 3000-8000`) now query the host's active listener table once in bulk and filter in memory, reducing 5,000-port scan times from ~42s down to ~20ms by avoiding thousands of redundant `lsof` process spawns on macOS and `/proc` parsing passes on Linux.
- **Refined exposure risk classification**: Accurately distinguishes private LAN/VPN addresses (RFC 1918 / RFC 4193 / link-local) from public internet-routable WAN interfaces.

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

- **Removed gopsutil and embedded SQLite.** Process metadata is now read
  natively on both platforms — `sysctl` + the raw `__sysctl` syscall + libproc
  (`proc_pidpath`/`proc_pidinfo`) on macOS, byte-oriented `/proc` on Linux —
  with no external commands and no hidden `ps` spawns (previously gopsutil ran
  `ps` twice per lookup on macOS). The binary shrank **9.3 MB → 5.9 MB** and
  `go.mod` went from ~19 modules to 3 (`purego`, `x/sys`, `x/term`).
- **History is now an owner-only (0600) JSONL log** with atomic `O_APPEND`
  appends instead of an embedded SQLite database, cutting a large dependency
  and its per-invocation open cost. The old `history.db` is left untouched;
  delete it once you no longer need it.
- **The macOS listing uses one lsof call** (`-FpctnT`, TCP LISTEN + UDP in a
  single spawn, protocol recovered from the `TST=` field) instead of two,
  halving the listing's external-process cost.
- **`--restart` re-runs the raw argv directly** (`exec.Command(argv[0],
  argv[1:]...)`) instead of `sh -c`, so a crafted argv cannot inject shell
  syntax. It also picks the *nearest* shell ancestor (nested shells such as
  Terminal → zsh → tool → zsh → target previously chose the wrong command) and
  re-resolves the launch argv when the ancestor chain only carries identity.
- `go test -race` is now run in CI on **Linux**; on macOS it remains blocked by
  the documented Go-1.23/macOS `dyld: missing LC_UUID` toolchain issue (which
  is why `CGO_ENABLED=0` is required), not by project code.
- **Performance: lazy inspection depth.** `portlens <port>` now runs a fast
  path that resolves ownership, minimal process metadata, project, exposure,
  and container — but skips the process tree, network connections, and verbose
  facts unless the requested output needs them (`--verbose`, `--tree`,
  `--connections`, single-port `--json`). This removed the expensive
  full-process scans and hidden `ps` spawns from the default lookup, cutting
  end-to-end latency roughly **10x** (≈250ms → ≈20ms on macOS) and
  allocations per inspection from ≈16,700 to ≈450.
- **Performance: native process tables.** Process hierarchy operations
  (Ancestors/Children/Descendants, used by `--verbose`, `--tree`, `--kill`,
  and `--pid`/`--name`) now read the process table once per invocation — a
  single `sysctl` on macOS, one `/proc` scan on Linux — instead of enumerating
  the process table repeatedly via gopsutil. Deep inspection dropped from
  ≈70ms to ≈32ms with ≈15x fewer allocations. gopsutil remains only for
  per-process metadata, and the fast path (`InfoBasic`) no longer spawns `ps`.
- **Multi-port `--json`** now uses the fast depth, so each array entry carries
  the essentials (port, protocol, status, address, service, process, origin,
  project, exposure, container) and omits the process tree and network
  sections.
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
