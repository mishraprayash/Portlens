# PortLens

**Local port intelligence & process management for developers.**

`portlens 3000` answers, in one glance:

> What is using this port? Where did it come from? What project does it belong
> to? What else is connected to it? Is it exposed beyond localhost? What happens
> if I terminate it? Can PortLens safely restart or open it?

PortLens inspects a local TCP/UDP port, resolves the owning process (or the
container that publishes it), walks its process hierarchy, detects the
project/runtime/framework it belongs to, summarizes its network connections,
flags exposure risks, and offers safe actions (graceful kill, restart, open in
browser, copy to clipboard) — all without ever transmitting data off your
machine.

---

## 1. Problem statement

Finding out what is running on a port usually means piecing together `lsof`,
`netstat`/`ss`, `ps`, and `git` by hand, then guessing what the process *is*,
what it belongs to, and whether it's safe to kill. That context — the command,
the parent process, the working directory, the git repo, the framework, the
exposure — is exactly what a developer needs before touching a process, and it
is scattered across a dozen different tools.

PortLens gathers that context into a single, fast, deterministic view and
distinguishes **facts** (things it observed) from **inferences** (things it
guessed). It never silently kills processes, never escalates privileges, and
never collects or transmits any data.

## 2. Installation

### Requirements

- Go 1.23+ (to build from source)
- macOS or Linux

### Quick install (from source)

```bash
git clone https://github.com/portlens/portlens.git
cd portlens
make install            # builds with CGO_ENABLED=0, installs to $GOBIN
```

Make sure `$GOBIN` (`~/go/bin`) is on your `PATH`, then:

```bash
$ portlens --version
portlens 0.1.0
```

### Other install methods

- **Single static binary** — `make build` produces `./bin/portlens`; copy it
  anywhere (`/usr/local/bin/portlens`).
- **Cross-compile for another platform** — `GOOS=linux GOARCH=amd64
  CGO_ENABLED=0 go build -o portlens .`

PortLens builds without cgo, so it is a single, static, dependency-free binary.

> **Note:** build with `CGO_ENABLED=0` (the Makefile does this automatically).
> A plain `go install .` with Go 1.23.x on recent macOS produces a binary that
> crashes with `dyld: missing LC_UUID load command`.

Full per-platform instructions (Go setup, PATH, Homebrew/Linux distro notes,
cross-compilation table, verification, uninstall, troubleshooting) are in
**[docs/installation.md](docs/installation.md)**.

## 3. Quick start

```bash
$ portlens 3000
```

```
PORT 3000
────────────────────────────────────────────
Status       LISTENING
Protocol     TCP
Address      127.0.0.1:3000

PROCESS
PID          48231
Name         node
Command      pnpm dev
Started      21:42:13
Runtime      1h 12m
User         prayash

PROJECT
Directory    ~/projects/orbit/backend
Git Repo     orbit-backend  (main)
Runtime      Node.js
Framework    NestJS
Package Mgr  pnpm

EXPOSURE
✓ LOW RISK  Bound only to loopback; not reachable from other machines

PROCESS TREE
launchd
└── zsh
    └── pnpm dev
        └── node  (pid 48231)  ← owns this port

NETWORK
Connections   8
ESTABLISHED   5
TIME_WAIT     3

INTERPRETATION
NestJS process (Node.js)

Facts
  Process 48231 (node) is listening on 127.0.0.1:3000 over tcp
  Full command: pnpm dev
  ...

Inferred  (best-effort, not guaranteed)
  Appears to be a NestJS process (Node.js)
  Belongs to git repository "orbit-backend" on branch "main"

ACTIONS
[k] Kill gracefully   [f] Force kill   [r] Restart   [o] Open localhost
[c] Copy PID          [u] Copy URL     [t] Tree      [n] Connections
[q] Quit
```

With no port, PortLens lists the interesting listening ports, identifying the
well-known **service** per port and whether each process is a **system**
(OS-bundled) or **user** (installed) component:

```bash
$ portlens
PORT   PROCESS               SERVICE        PROJECT  RUNTIME     PROTOCOL  ADDRESS         STATUS  ORIGIN
88     kdc                   Kerberos       -        -           tcp       0.0.0.0:88      LISTEN  system
5353   mDNSResponder         mDNS (DNS-SD)  -        -           udp       0.0.0.0:5353     BOUND   system
5432   postgres              PostgreSQL     brew     PostgreSQL  tcp       [::]:5432        LISTEN  user
6379   redis-server          Redis          brew     Redis       tcp       127.0.0.1:6379   LISTEN  user
```

A `CONTAINER` column appears when a port is owned by a container (see
[Docker & container awareness](#42-docker--container-awareness)); `--filter`
also matches the service and origin columns.

## 4. Command reference

```
portlens <port>...                 Inspect port(s) — compact summary by default
portlens <port>... --verbose       Full detailed report (-v)
portlens 4000-4010                 Scan a range: only in-use ports are printed,
                                   with live progress, ETA, and a final summary
portlens @dev                      Inspect a named group from your config
portlens                           List interesting listening ports
portlens <port>... --tree          Show the complete process hierarchy
portlens <port>... --connections   Show network connections, grouped & summarized
portlens <port>... --json          JSON output — for a range/multiple ports this
                                   emits the in-use ports as an array (progress
                                   and ETA on stderr, so stdout stays pipeable)
portlens <port>... --log <file>    Write this command's output to <file> — works
                                   for any command, including --json; plain text,
                                   no progress lines, interactive mode disabled
portlens <port>... --kill          Gracefully terminate the owning process(es) (SIGTERM)
portlens <port>... --kill --force  Force termination (SIGKILL)
portlens <port>... --restart       Restart the process if the launch command is known
portlens <port>... --history       Show previously observed activity on this port
portlens <port>... --open          Open the service in your browser
portlens --all                     Act on every listening port (e.g. --all --kill)
portlens --pid <pid>               Find the listening ports owned by a process (incl. descendants)
portlens --name <query>            Find ports by process name/command (regex with /.../)
portlens <port>... --watch         Re-render every --interval seconds until Ctrl-C
portlens <port>... --watch --notify  Notify when a port goes up, down, or changes
portlens config                    Manage named port groups (@name)
portlens --version                 Print the version
portlens --help                    Show help
```

Listing flags: `--sort <port|process|project|runtime>`, `--filter <text>`,
`--tcp`.

Watch flags: `--interval <secs>` (default 1), `--notify` (desktop notification
on state change; requires `--watch`).

General flags: `--protocol <tcp|udp>`, `--yes`/`-y` (skip confirmations),
`--verbose`/`-v` (full detailed report instead of the compact summary),
`--debug`/`-d` (structured diagnostic logging to stderr; also honors `PORTLENS_DEBUG=1`),
`--log <path>` (capture any command's stdout to a file; plain text, disables
interactive mode, incompatible with `--watch`), `--no-color`, `--no-record`
(skip history recording), `--no-docker` (skip container detection).

## 4.1 Usage & use cases

For step-by-step examples and a use case for **every** feature — inspection,
listing, tree, connections, JSON, kill/force-kill, restart, open, history,
clipboard, interactive keys, exit codes, UDP, **port-range scanning with
progress/ETA and `--log`**, and Docker awareness — see the
[Usage guide & use cases](docs/usage.md).

## 4.2 Docker & container awareness

When a port is published by a container (Docker Desktop, Docker Engine, or a
Compose stack), PortLens shows the container alongside the process:

```
PORT 8080
───────────────
Status      LISTENING
Protocol    TCP
Address     127.0.0.1:8080
Process     docker-proxy (pid 48231)
Container   api-1 (nginx:alpine, api)
Exposure    WARNING
```

- **Detection** — the listing gains a `CONTAINER` column and the full report a
  `CONTAINER` section (name, ID, image, compose project/service, status).
  Linux resolves the owning process's cgroup first (a kernel fact that needs no
  daemon); everywhere, a single query to the local Docker daemon (over its unix
  socket, honoring `DOCKER_HOST`) maps host ports to containers.
- **Container-aware actions** — `--kill` and `--restart` on a containerized
  port stop/restart the **container**, not the host-side process. On macOS the
  host-side process is the Docker VM, which must never be signaled — so this is
  both more correct and safer.
- **Graceful degradation** — when no Docker daemon is reachable, container
  detection is skipped silently; nothing breaks. Disable it explicitly with
  `--no-docker`.
- **Local-first** — only the local daemon socket is ever queried. Nothing
  leaves your machine.

## 5. Supported operating systems

| OS      | Status       | Port/connection source        |
|---------|--------------|-------------------------------|
| macOS   | Supported    | `lsof` (isolated behind the abstraction layer) |
| Linux   | Supported    | `/proc/net/*` + `/proc/<pid>/fd` (native, no external commands) |
| Windows | Planned      | Architecture is ready; a `windows_*.go` implementation can be added |

## 6. Security & privacy model

PortLens is **local-first** by design:

- **No telemetry.** Nothing is reported anywhere, ever.
- **No cloud service.** There is no server to talk to.
- **No data exfiltration.** Process information never leaves your machine.
- **Secrets are never printed.** Environment variable *values* are never shown;
  only well-known launchd/systemd `KEY=value` argv artifacts are stripped from
  the human display (the raw command line remains available via `--json`).
- **No silent destructive actions.** Kill and restart always confirm first
  unless explicitly bypassed with `--yes`/`--force`.
- **No privilege escalation.** If a process requires elevated privileges to
  inspect or signal, PortLens reports the permission problem rather than trying
  to escalate.
- **Local history only.** History is stored in a local, owner-only (0600)
  JSONL log file under your OS data directory and is never transmitted.

## 7. Architecture overview

PortLens is layered around an OS abstraction. Platform-specific behavior is
confined to build-tagged files; everything else is OS-independent.

```
cmd/                 CLI entrypoint, argument parsing, exit-code mapping
internal/model/      Shared, OS-independent data types
internal/platform/   The abstraction layer (interfaces + factories)
  interfaces:        PortResolver, ProcessInspector, NetworkInspector,
                     ProcessTreeProvider, ClipboardProvider, ProcessController
  darwin_*.go        macOS implementations (sysctl + libproc, lsof fallback,
                     pbcopy, syscall)
  linux_*.go         Linux implementations (/proc, xclip/wl-copy, syscall)
internal/inspector/  Orchestrates providers into a Report (+ risk assessment)
internal/detect/     Project / runtime / framework detection (filesystem + argv)
internal/history/    Local owner-only JSONL history log
internal/actions/    Kill / restart / open / copy (with confirmation)
internal/render/     Terminal UI, tables, tree, JSON output
tests/               Integration tests using controlled processes
```

Key interfaces (see `internal/platform/platform.go`):

```
ProcessInspector, NetworkInspector, PortResolver,
ProcessTreeProvider, ClipboardProvider, ProjectDetector,
HistoryStore, ProcessController
```

See [docs/architecture.md](docs/architecture.md) for details.

### Documentation index

| Document                          | Contents                                            |
|-----------------------------------|-----------------------------------------------------|
| [docs/installation.md](docs/installation.md) | Per-platform install, cross-compile, uninstall |
| [docs/usage.md](docs/usage.md)           | Usage guide & use cases for every feature            |
| [docs/architecture.md](docs/architecture.md) | Layered design & the OS abstraction            |
| [docs/json-schema.md](docs/json-schema.md) | Deterministic `--json` schema                   |
| [docs/exit-codes.md](docs/exit-codes.md)   | Exit codes for scripting                        |
| [docs/detection.md](docs/detection.md)     | Facts vs. inferences; how detection works       |
| [docs/security.md](docs/security.md)       | Security & privacy model                        |
| [docs/performance.md](docs/performance.md) | Fast/deep paths, benchmarks, syscall & alloc analysis |

## 8. Development

```bash
make build          # build to ./bin/portlens
make lint           # gofmt check + go vet
make test           # run all tests (unit + integration, no cache)
make check          # lint + test — the same gate CI runs
make fmt            # gofmt
make cross          # verify macOS/Linux cross-compilation
```

The test suite spins up controlled processes (not arbitrary processes on your
machine) for integration coverage. See `tests/integration`.

### Note on cgo

PortLens builds with `CGO_ENABLED=0`. This keeps binaries static, makes
cross-compilation trivial, and avoids a linker incompatibility between older Go
toolchains and recent macOS releases. The `Makefile` sets this automatically.

## 9. Roadmap

- **Windows support** — implement `windows_*.go` providers (the abstraction
  layer is already in place).
- **Richer framework detection** — more runtimes and frameworks (Java
  application servers, .NET, etc.).
- **Open-files view** — list open files/descriptors for the owning process.
- **CPU/memory sampling** — optional live sampling in the interactive view.
- **Profile-aware restarts** — restart using the detected package manager and
  script name rather than the raw command line.
- **Shell completions** — bash/zsh/fish completion scripts.

## 10. Contributing

Contributions are welcome. Please read
[CONTRIBUTING.md](CONTRIBUTING.md) first — it covers the project principles,
the layered architecture, code conventions, and how a feature maps to the
codebase. We also have a [Code of Conduct](CODE_OF_CONDUCT.md) and a
[Security policy](SECURITY.md).

The short version:

```bash
make check          # gofmt check + go vet + full test suite
```

Open an issue before starting, keep changes focused with tests, update
`CHANGELOG.md` under `[Unreleased]`, and make sure CI passes.

## 11. License

MIT — see [LICENSE](LICENSE).
