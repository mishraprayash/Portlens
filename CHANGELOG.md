# Changelog

All notable changes to PortLens are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and this project adheres to
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed

- **Module path matches the canonical repository**: `go.mod` and every import
  now declare `github.com/mishraprayash/Portlens` (previously
  `github.com/portlens/portlens`, which does not exist), so `go install
  github.com/mishraprayash/Portlens@latest` and the documented clone URL work.
- **Subcommands no longer mistake flag values for port targets**: `portlens
  kill --filter node`, `tree --sort process`, `inspect --protocol udp`, and
  similar invocations now fail with exit code 2 and a usage message instead of
  printing the default listing and exiting 0 without acting.
- **Action flags without a port target fail fast**: `portlens --kill`,
  `--tree`, `--restart`, `--open`, and `--connections` without a port,
  `--all`, `--pid`, or `--name` now return exit code 2 instead of silently
  degrading to the full listing.
- **`--sort` key validation**: an unknown `--sort` value (e.g. `--sort pid`)
  is rejected with exit code 2 instead of being silently ignored and falling
  back to port order.
- **`next --protocol` validation**: an unknown protocol now fails with exit
  code 2 instead of silently falling back to TCP.
- **`top` reports unknown flags and stray arguments**: typos such as
  `portlens top --filter x` fail with exit code 2 instead of being dropped,
  and a missing `--interval` value is an error rather than silently using the
  default.
- **Permission errors exit with code 4**: signalling a process owned by
  another user (EPERM) now exits 4 as `docs/exit-codes.md` promises — the
  platform and model `ErrPermissionDenied` are a single sentinel, so
  `model.MapExitCode` recognizes it. Previously it fell through to exit 1.
- **IPv6 addresses are bracketed in every human-facing view**: the TUI
  overview, TUI connections table, and inspector facts now render
  `[::1]:5432` via the shared `model.FormatAddr` instead of `::1:5432`.
  JSON output still carries the raw address.
- **One filter predicate for the table and the TUI**: `--filter` and the TUI's
  `/` filter now both use `model.PortEntry.Matches`, a case-insensitive
  substring match over port, process, service, project, runtime, address,
  status, origin, protocol, and container fields (queries are trimmed of
  surrounding whitespace). Previously the two implementations matched
  different field sets — e.g. `--filter listening` did not work in the TUI
  and `--filter tcp` did not work in the table.
- **Watch mode on Linux sees processes started after launch**: the shared
  socket-inode→PID cache expires after one second instead of being built
  once per invocation, so long-running `--watch` sessions attribute new
  processes correctly (single-shot commands still scan at most once).
- **Shell completion cannot hang**: the dynamic `--_complete_ports` listener
  scan is bounded by a 10-second deadline derived from the caller's
  context; a canceled or expired context yields no completions and still
  exits 0.
- **`--watch` rejects action flags**: `portlens watch --kill 3000` (or
  `--watch --tree`, `-w -k`, ...) now fails with exit 2 and
  `--watch cannot be combined with --kill, --restart, --open, --tree, or
  --connections` instead of silently starting the monitor and ignoring the
  action.
- **`parseLsofFields` multi-process socket attribution**: Fixed a critical bug in `lsof` field parsing where `flush()` was not called upon encountering a new PID line, causing the trailing socket of any process to be misattributed to the subsequent process.
- **Detached process termination on exit in `--restart`**: Replaced `exec.CommandContext` with `exec.Command` in `defaultProcessStarter` so that Go's runtime context monitor does not terminate the restarted detached process when PortLens completes execution.
- **Interactive mode nil pointer dereference panic**: Guarded against nil `report.Process` when pressing `'c'` to copy PID, preventing panics on unprivileged or system listeners.
- **IPv6 HTTP endpoint probing (`::1`)**: Used `net.JoinHostPort` to properly bracket IPv6 hosts in HTTP probe requests and enabled `DisableKeepAlives` on probe transports to prevent socket descriptor leaks.
- **`mDNSResponder` origin identification and Linux `/home/` support**: Lowercased `"mdnsresponder"` in `systemProcessNames` to match case-folded process names, and added `"/home/"` to `userPathPrefixes` for Linux user binary detection.
- **`find` subcommand `--pid=` and `--name=` flag support**: Added support for inline `--flag=value` syntax in `portlens find`.
- **UDP container matching in port listings**: Relaxed protocol filtering in `attachContainers` so UDP-published container services are properly mapped to their container metadata in listings.
- **Cgroup scanner false-positive matching on long hex strings**: Fixed `scanContainerID` offset tracking so that 128-hex tokens (such as SHA-512 hashes) are not partially matched as 64-hex container IDs.
- **Performance optimizations in process inspection and rendering**: Eliminated redundant `/proc/<pid>/stat` reads on Linux, duplicate `darwinArgs` syscalls on macOS, duplicate dependency instantiation in `service.New`, shared the Linux socket inode cache between resolvers, and replaced string concatenation churn in `kv` rendering and list filtering with efficient implementations.
- **`--restart` now gracefully terminates the existing process first**: Shuts down the running process tree and waits for the port to release before relaunching, eliminating `EADDRINUSE` socket conflicts. Stdio is detached into `/dev/null` so background logs do not corrupt the terminal session.
- **`LaunchProcess` directly launched by shell**: Fixed detection so that when a process is the immediate child of an interactive shell, PortLens restarts the process itself rather than mistakenly trying to re-execute the parent shell.
- **Subcommand routing with global flags**: `portlens [flags] config ...` (e.g. `portlens --no-color config list`) now dispatches to the config subcommand instead of erroring with an invalid port.
- **Git worktrees, submodules, and branch paths**: Supports `.git` files with `gitdir:` indirection, preserves full multi-segment branch names (e.g. `feature/auth/oauth2`), parses SSH remote URLs, and handles detached HEAD states.
- **Interactive TUI escape sequence drainage & PID verification**: Drains trailing bytes of ANSI escape sequences (e.g. arrow keys) so they do not spill into the shell upon exit, and confirms the target PID is still alive before signaling to prevent killing recycled PIDs.
- **Watch mode immediate signal cancellation**: Migrated `runWatch` to `signal.NotifyContext` so `Ctrl-C` or `SIGTERM` cancels in-flight inspection passes immediately instead of blocking until the current tick finishes.
- **Unbounded `lsof` execution timeout**: Added a 10-second safety deadline to macOS `runLsof` when invoked with an unbounded context, preventing indefinite blocking on unresponsive network mounts (NFS/SMB).
- **Linux `/proc` socket link slice bounds safety**: Added defensive validation for socket symlinks in `/proc/<pid>/fd`, eliminating potential slice out-of-bounds panics on malformed symlinks.
- **macOS `lsof` exit code 1 handling**: Fixed `runLsof` to treat exit code 1 as successful even when partial output is emitted alongside non-fatal warnings (e.g. unprivileged execution), preventing `portlens list` from failing on unprivileged runner environments.
- **Non-blocking browser launching on Linux**: Launched browser processes asynchronously via `cmd.Start()`, preventing foreground browser processes from locking the PortLens CLI.

### Changed

- **Subcommand dispatch de-duplicated**: the eleven copies of the per-command
  `--help` scan, the port-target pre-checks, and the action-flag injection
  now flow through shared `wantsHelp`/`runSimple` helpers, and `config`
  load/save failures go through `loadConfig`/`saveConfig`. Behavior and
  messages are unchanged; new subcommands need a one-line declaration.
- **CLI error paths share one `fail` helper**: every error message in `cmd`
  is written and its exit code returned from a single call site, so message
  and code cannot drift apart; `model.MapExitCode` now has unit coverage of
  every branch in the `docs/exit-codes.md` table.
- **`make check` now matches the CI gate**: it runs gofmt, vet, build, the full
  test suite, and the four-target cross-compile matrix — the same checks CI
  runs. `make cover` (new) prints an aggregate coverage percentage, and CI
  reports coverage on the Linux job.
- **Docs refreshed for the post-refactor architecture**: the data flow in
  `docs/architecture.md` now routes through `service.PortService`; the project
  layout blocks list `internal/tui`; the removed `HistoryStore` interface and
  the shipped shell-completion roadmap item were dropped; and `CHANGELOG.md`'s
  duplicate `[Unreleased]` sections were merged (contradictory history/`--log`
  entries pruned).
- **Enterprise-grade Clean Architecture and Functional Options**: Decoupled core business domain from CLI delivery through a new `internal/service.PortService` facade. Refactored `inspector.New` and `render.NewRenderer` to use Functional Options and consumer-defined interfaces (`PortInspector`). Centralized domain sentinel errors in `internal/model/errors.go` and threaded signal context cancellation (`osSignalContext` / `ExecuteContext`) through all I/O and worker pool boundaries.
- **Standard open-source subcommand CLI architecture**: Elevated primary actions to intuitive, first-class subcommands (`portlens kill`, `portlens list`/`ls`, `portlens inspect`, `portlens watch`, `portlens find`, `portlens tree`, `portlens conn`, `portlens open`, `portlens restart`) while preserving 100% backward compatibility with flag-based invocations (`portlens 3000 --kill`, `portlens 3000 -t`).
- **Structured and modernized help output**: Redesigned `portlens --help` and added subcommand-specific `--help` usage screens following open-source CLI standards.
- **Parallel multi-port and range scan inspection**: Replaced sequential port inspection in `scanPorts` with a concurrent worker pool (`min(2*NumCPU, 16)` workers), parallelizing inspection across active ports while maintaining exact input order and thread-safe progress reporting.
- **Multi-port and range scan bulk pre-filtering**: Scans (`portlens 3000-8000`) now query the host's active listener table once in bulk and filter in memory, reducing 5,000-port scan times from ~42s down to ~20ms by avoiding thousands of redundant `lsof` process spawns on macOS and `/proc` parsing passes on Linux.
- **Refined exposure risk classification**: Accurately distinguishes private LAN/VPN addresses (RFC 1918 / RFC 4193 / link-local) from public internet-routable WAN interfaces.

- **Enterprise-grade Clean Architecture and Functional Options**: Decoupled core business domain from CLI delivery through a new `internal/service.PortService` facade. Refactored `inspector.New` and `render.NewRenderer` to use Functional Options and consumer-defined interfaces (`PortInspector`). Centralized domain sentinel errors in `internal/model/errors.go` and threaded signal context cancellation (`osSignalContext` / `ExecuteContext`) through all I/O and worker pool boundaries.
- **Standard open-source subcommand CLI architecture**: Elevated primary actions to intuitive, first-class subcommands (`portlens kill`, `portlens list`/`ls`, `portlens inspect`, `portlens watch`, `portlens find`, `portlens tree`, `portlens conn`, `portlens open`, `portlens restart`) while preserving 100% backward compatibility with flag-based invocations (`portlens 3000 --kill`, `portlens 3000 -t`).
- **Structured and modernized help output**: Redesigned `portlens --help` and added subcommand-specific `--help` usage screens following open-source CLI standards.
- **Parallel multi-port and range scan inspection**: Replaced sequential port inspection in `scanPorts` with a concurrent worker pool (`min(2*NumCPU, 16)` workers), parallelizing inspection across active ports while maintaining exact input order and thread-safe progress reporting.
- **Multi-port and range scan bulk pre-filtering**: Scans (`portlens 3000-8000`) now query the host's active listener table once in bulk and filter in memory, reducing 5,000-port scan times from ~42s down to ~20ms by avoiding thousands of redundant `lsof` process spawns on macOS and `/proc` parsing passes on Linux.
- **Refined exposure risk classification**: Accurately distinguishes private LAN/VPN addresses (RFC 1918 / RFC 4193 / link-local) from public internet-routable WAN interfaces.

- **Removed gopsutil and embedded SQLite.** Process metadata is now read
  natively on both platforms — `sysctl` + the raw `__sysctl` syscall + libproc
  (`proc_pidpath`/`proc_pidinfo`) on macOS, byte-oriented `/proc` on Linux —
  with no external commands and no hidden `ps` spawns (previously gopsutil ran
  `ps` twice per lookup on macOS). The binary shrank **9.3 MB → 5.9 MB** and
  `go.mod` went from ~19 modules to 3 (`purego`, `x/sys`, `x/term`).
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
  ≈70ms to ≈32ms with ≈15x fewer allocations. The fast path (`InfoBasic`)
  no longer spawns `ps`.
- **Multi-port `--json`** now uses the fast depth, so each array entry carries
  the essentials (port, protocol, status, address, service, process, origin,
  project, exposure, container) and omits the process tree and network
  sections.
- `--json` on a range or multiple ports now emits **only the in-use ports** as
  an array (idle ports are omitted, matching scan mode) and shows the same
  scan progress/ETA/summary on stderr, so stdout stays a pure JSON payload
  ready for `jq` or a file.
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

### Removed

- **Removed `--log <file>`**: Removed internal stdout-teeing flag in favor of standard Unix redirection and pipes (`portlens 3000 > out.txt`, `portlens 3000 | tee out.txt`), simplifying flag handling and adhering to Unix philosophy.
- **Removed local observation history (`--history`, `--no-record`, `internal/history`)**: Removed disk-based invocation logging, making PortLens completely stateless and zero-footprint, eliminating disk write overhead during scans, and removing privacy concerns around saving command arguments to disk.

### Added

- **Testing guide (`docs/testing.md`)**: documents the `CGO_ENABLED=0`
  requirement (a bare `go test ./...` fails on macOS), the coverage and race
  targets, test layout rules, and the cross-compile matrix. CONTRIBUTING's
  testing section now links to it and warns about the same trap.
- **Full-Screen Interactive TUI Dashboard (`portlens top` / `portlens tui`)**: Added a zero-dependency, double-buffered full-screen terminal dashboard built directly on `golang.org/x/term` and standard ANSI escape sequences (`\x1b[?1049h`). Features split-pane navigation with real-time port selection, live search/filtering (`/`), instant tab switching (`1` Overview, `2`/`t` Process Tree, `3`/`n` Connections), safe in-place action triggers with confirmation modals (`k` graceful kill, `f` force kill, `r` restart, `o` browser open, `c`/`u` clipboard copy), background polling, responsive layout resizing on `SIGWINCH`, and guaranteed fail-safe terminal cleanup on any exit path.
- **Shell autocompletion generator (`portlens completion <bash|zsh|fish>`)**: Generates dynamic shell autocompletion for bash, zsh, and fish that completes subcommands, flags, and currently active listening ports with their process names.
- **HTTP health & HTML title probing (`--probe` / `-p`)**: Lightweight HTTP probing with a 300ms timeout extracts HTTP status, response latency, Server header, and HTML `<title>` to immediately identify the web application running behind generic process names.
- **Process Memory RSS display**: Surfaced native process memory usage (formatted as human-friendly RSS units e.g. `128 MB`, `1.4 GB`) in both compact summary and full verbose reports.
- **`portlens next [start]` available port discovery subcommand**: Find the lowest unused and bindable listening port (defaults to 3000, or a user-specified starting port) for scripting and automation.
- **Podman & rootless container runtime awareness**: Expanded container detection beyond Docker to Podman, supporting `CONTAINER_HOST` environment variables, system/rootful sockets (`/run/podman/podman.sock`), user rootless sockets (`$XDG_RUNTIME_DIR/podman/podman.sock`, `/run/user/<uid>/podman/podman.sock`), macOS Podman machine sockets, and cgroup v2 systemd/libpod scopes.
- **Modular CLI subcommand architecture**: Introduced a decoupled `SubcommandRegistry` and `Subcommand` routing layer in `cmd/dispatch.go`, cleanly separating top-level subcommands and their aliases from port arguments while cleanly handling preceding global flags.
- **Structured diagnostic logging (`--debug`, `-d`, or `PORTLENS_DEBUG=1`)**: Added structured `log/slog` debug tracing across the CLI, inspector, detector, and platform layers to `stderr`, enabling deep field troubleshooting without cluttering standard output.
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
