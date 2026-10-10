# Usage guide & use cases

This guide walks through every PortLens feature with realistic, copy-pasteable
examples. Run `portlens --help` for the full flag reference.

---

## Inspecting a port — `portlens <port>`

The core command. Point it at any port to learn who owns it, where it came from,
what it belongs to, how it's exposed, and what actions are safe. By default it
shows a **compact summary** of the important facts; add `--verbose` (or `-v`)
for the full detailed report.

```bash
$ portlens 5432
PORT 5432
Status     LISTENING
Address    [::]:5432
Service    PostgreSQL
Process    postgres (pid 946)
Command    postgres -D /opt/homebrew/var/postgresql@14
Origin     user
Project    brew  (PostgreSQL)
Exposure   WARNING
```

```text
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

**Use case — "what is using 8080?"** You get the answer plus the project it
belongs to and the command that started it, without running `lsof` + `ps` +
`git` by hand.

**Use case — "is my dev server exposed?"** The `EXPOSURE` section answers this.
`127.0.0.1` is loopback; `0.0.0.0` / `::` means it is bound to all interfaces.

On an interactive terminal, PortLens enters a single-key action loop after the
report. On a piped/redirected terminal, it prints the report once and exits
(plain text, no colors).

---

## Listing ports — `portlens` (no argument)

```bash
$ portlens
PORT   PROCESS               SERVICE        PROJECT  RUNTIME     PROTOCOL  ADDRESS         STATUS  ORIGIN
88     kdc                   Kerberos       -        -           tcp       0.0.0.0:88      LISTEN  system
5353   mDNSResponder         mDNS (DNS-SD)  -        -           udp       0.0.0.0:5353     BOUND   system
5432   postgres              PostgreSQL     brew     PostgreSQL  tcp       [::]:5432        LISTEN  user
6379   redis-server          Redis          brew     Redis       tcp       127.0.0.1:6379   LISTEN  user
27017  mongod                MongoDB        brew     MongoDB     tcp       127.0.0.1:27017  LISTEN  user
```

**Use case — "what's running on this machine right now?"** A quick inventory of
listening services. The columns answer the three questions that matter:

- **SERVICE** — what the port is *for* from a well-known-port registry
  (`5432 → PostgreSQL`, `5353 → mDNS`); `-` when the port has no registry entry.
- **PROTOCOL** — `tcp` or `udp`. A port used by both (e.g. `88` Kerberos) shows
  as two rows; `LISTEN`/`BOUND` correspond to TCP/UDP respectively.
- **ORIGIN** — whether the owning process is **system** (bundled with the OS:
  `kdc`, `mDNSResponder`, `rapportd`, ...) or **user** (Homebrew services,
  `/Applications` apps, language toolchains). This is a best-effort heuristic;
  `-` when unknown.

### Sorting & filtering

```bash
$ portlens --sort process        # order by process name
$ portlens --sort project        # group by detected project
$ portlens --sort runtime        # group by runtime (node, go, python, ...)
$ portlens --filter node         # only rows matching "node"
$ portlens --filter orbit        # find every port owned by the "orbit" project
$ portlens --filter system       # only OS-bundled processes (origin == system)
$ portlens --filter kerberos     # find the service by its well-known name
$ portlens --tcp                 # TCP listeners only (hide UDP)
```

**Use case — "which ports does my `orbit` project occupy?"**
`portlens --filter orbit` answers it instantly.

**Use case — "are any of my processes the OS's?"** `portlens --filter system`
lists only OS-bundled components; everything else on your machine is
user-installed (Homebrew, apps, toolchains).

---

## Process hierarchy — `portlens <port> --tree`

```bash
$ portlens 5432 --tree
PROCESS TREE — PID 946
────────────────────────────────────────────
launchd
└── postgres -D /opt/homebrew/var/postgresql@14  (pid 946)  ← owns this port
    ├── postgres: checkpointer
    ├── postgres: background writer
    ├── postgres: walwriter
    └── postgres: autovacuum launcher
```

**Use case — "where did this process come from?"** The complete ancestor chain
(`launchd` → `zsh` → `pnpm dev` → `node`) reveals whether it was started by
your shell, a launch agent, or a service manager.

**Use case — "what will I kill?"** See the full descendant tree before
terminating so you know the blast radius.

---

## Network connections — `portlens <port> --connections`

```bash
$ portlens 3000 --connections
CONNECTIONS — PID 48231
────────────────────────────────────────────
Total   8

ESTABLISHED   5
TIME_WAIT     3

LOCAL          REMOTE         STATE
127.0.0.1:3000 127.0.0.1:52000 ESTABLISHED
127.0.0.1:3000 127.0.0.1:52001 ESTABLISHED
...
```

**Use case — "who is talking to my server?"** See remote addresses and states,
grouped and summarized instead of a raw dump.

**Use case — "is anything still connected before I kill it?"** If `ESTABLISHED`
is non-zero, active clients will be disconnected when you terminate.

---

## Machine-readable output — `portlens <port> --json`

```bash
$ portlens 3000 --json > report.json
$ portlens 3000 --json | jq '.process.pid, .project.framework'
48231
"nestjs"
```

```json
{
  "port": 3000,
  "protocol": "tcp",
  "status": "listening",
  "address": "127.0.0.1",
  "process": { "pid": 48231, "name": "node", "command": "pnpm dev" },
  "project": { "name": "orbit-backend", "runtime": "node", "framework": "nestjs" },
  "network": { "address": "127.0.0.1", "connections": [] },
  "exposure": { "worst_level": "LOW RISK" }
}
```

A single port emits one object (including the `not_listening` shape when idle).
A range or several ports emit a JSON **array of only the in-use ports** — idle
ports are omitted, matching scan mode. Progress and the `Found N of M`
summary go to **stderr**, so stdout is always a pure JSON payload:

```bash
$ portlens 4000-4010 --json | jq 'length'     # counts only the in-use ports
$ portlens 4000-4010 --json > scan.json        # clean array, no progress in it
```

**Use case — shell scripts.** Parse the deterministic schema with `jq`.

**Use case — AI agents / tooling.** The stable, documented schema (see
[json-schema.md](json-schema.md)) is designed for programmatic consumption.

---

## Stopping a process — `--kill` and `--kill --force`

Graceful first, never `kill -9` by default.

```bash
$ portlens 3000 --kill
# shows the report, then:
Terminating process 48231 (node) and 2 descendant(s)
Send SIGTERM to the process tree? [y/N] y
Sending SIGTERM...
Process 48231 exited gracefully
```

```bash
$ portlens 3000 --kill --force      # skip the prompt, send SIGKILL immediately
$ portlens 3000 --kill --yes        # skip the prompt, still graceful (SIGTERM)
```

**Use case — "a dev server is stuck and won't shut down."** `--kill` sends
`SIGTERM` and waits; if the process still lives, PortLens tells you so and you
can escalate with `--force`.

**Use case — scripting a teardown.** Use `--kill --yes` for a graceful,
non-interactive stop, or `--kill --force` when you don't care about cleanup.

**Safety.** Confirmation is required unless you pass `--yes`/`--force`. If stdin
is not a terminal, PortLens refuses rather than guessing. It never escalates
privileges — if the process is owned by another user, it reports
"permission denied" (exit code 4) instead of trying `sudo`.

---

## Restarting a process — `portlens <port> --restart`

```bash
$ portlens 3000 --restart
Detected command:
  pnpm dev

Restart command:
  pnpm dev

[r] Restart
[c] Cancel
```

**Use case — "restart my dev server from anywhere."** PortLens re-runs the
detected launch command in the process's working directory.

**Honesty.** If the process wasn't launched from an interactive shell (e.g. a
launchd/systemd service), PortLens does **not** guess:

```bash
$ portlens 6379 --restart
Automatic restart is unavailable.
The process was not launched from an interactive shell in a way PortLens can reproduce.
```

(exit code 5)

---

## Docker & container awareness

When a port is published by a container (Docker Desktop, Docker Engine, or a
Compose stack), PortLens detects it and shows the container next to the
process:

```bash
$ portlens 8080
PORT 8080
Status      LISTENING
Protocol    TCP
Address     127.0.0.1:8080
Process     docker-proxy (pid 48231)
Container   api-1 (nginx:alpine, api)
Exposure    WARNING
```

The verbose report adds a `CONTAINER` section with the container name, ID,
image, status, and compose project/service, and the listing gains a `CONTAINER`
column (the column appears only when at least one port belongs to a container):

```bash
$ portlens
PORT   PROCESS       CONTAINER   PROJECT   RUNTIME   ADDRESS        STATUS
6379   docker-proxy  redis-1      -         -         127.0.0.1      LISTEN
```

**Container-aware kill & restart.** `--kill` and `--restart` act on the
**container** rather than the host-side process:

```bash
$ portlens 8080 --kill
Stopping container api-1
Stop container api-1? [y/N] y
Container api-1 stopped
```

This is more correct **and** safer: on macOS the host-side process behind a
container port is the Docker VM itself, which must never be signaled. The
graceful kill still confirms first; `--kill --force` stops the container with
SIGKILL without prompting.

**How detection works.** On Linux, the owning process's cgroup is consulted
first (a kernel fact — no daemon round-trip). In every case, one query to the
local Docker daemon (over its unix socket, honoring `DOCKER_HOST`) maps the
host port to a container. When no daemon is reachable, detection is skipped
silently — nothing breaks. Disable it explicitly with `--no-docker`.

---

## Opening in a browser — `portlens <port> --open`

```bash
$ portlens 3000 --open
Opening http://localhost:3000
```

**Use case — "jump straight to the running app."** The URL uses the detected
bind address (`127.0.0.1`/`::1`/wildcard → `localhost`; a specific interface
address is used verbatim).

**Honesty.** If the process doesn't look like an HTTP server, PortLens warns you
before opening, rather than silently launching a browser at a DB port.

---


## Copy to clipboard (interactive)

In the interactive view:

| Key | Action            |
|-----|-------------------|
| `c` | Copy the PID      |
| `u` | Copy the local URL |

**Use case — "grab the PID for a debugger or `kill`."** Press `c` and paste the
PID anywhere. Uses `pbcopy` (macOS) or `wl-copy`/`xclip`/`xsel` (Linux), and
fails gracefully if none is available.

---

## Interactive action keys

When `portlens <port>` runs on a terminal:

| Key | Action                              |
|-----|-------------------------------------|
| `k` | Kill gracefully (SIGTERM)           |
| `f` | Force kill (SIGKILL)                |
| `r` | Restart (if launch command is known)|
| `o` | Open in browser                     |
| `c` | Copy PID                            |
| `u` | Copy local URL                      |
| `t` | Show process tree                   |
| `n` | Show connections                    |
| `q` | Quit                                |

---

## Exit codes in scripts

```bash
if portlens 3000 >/dev/null 2>&1; then
  echo "something is listening on 3000"
fi

portlens 9999; echo $?   # 3 — port not found

if ! portlens 3000 --kill --yes; then
  case $? in
    4) echo "need elevated privileges";;
    5) echo "did not exit; try --kill --force";;
  esac
fi
```

See [exit-codes.md](exit-codes.md) for the full table.

---

## UDP inspection

```bash
$ portlens 5353 --protocol udp
```

**Use case — "what's listening on the mDNS/QUIC/DNS port?"** UDP sockets are
reported as `BOUND` (there is no UDP listen state). `--protocol` accepts
`tcp` or `udp` and defaults to both.

---

## Scripting flags

| Flag              | Effect                                          |
|-------------------|-------------------------------------------------|
| `--json` / `-j`   | Machine-readable JSON output                    |
| `--yes` / `-y`    | Skip confirmations (still graceful)             |
| `--force` / `-f`  | With `kill`: force SIGKILL, skip confirmation   |
| `--no-color`      | Plain output (no ANSI escapes)                  |
| `--no-docker`     | Disable container detection                     |
| `--probe` / `-p`  | Probe HTTP endpoint for status, title, & server |
| `--debug` / `-d`  | Structured diagnostic debug logging to stderr   |
| `--protocol`      | Restrict to `tcp` or `udp`                      |

**Use case — CI or non-interactive tooling.** Combine `--json --no-color` for
clean, parseable output.

---

## Port ranges and scanning

Multiple ports, ranges, and named groups can be mixed in one invocation:

```bash
$ portlens 3000 4000 5000            # a few ports
$ portlens 3000-3010                 # a range
$ portlens kill 4000-4010 --yes      # kill everything in a range
$ portlens @dev                      # a group from your config
$ portlens kill --all --force        # stop every listening process
```

### Scan mode

Inspecting several ports at once (a range, a group, or explicit ports) runs in
**scan mode**: instead of printing every port's details, PortLens shows live
progress and prints only the ports that are actually in use.

```bash
$ portlens 3000-8000
Scanning 5001 ports (3000-8000)...
PORT   PROCESS       PROJECT   RUNTIME    ADDRESS         STATUS
5432   postgres      brew      PostgreSQL [::]:5432       listening
6379   redis-server  brew      Redis      127.0.0.1:6379  listening

Found 2 of 5001 ports in use in 42.1s.
```

- **Progress & ETA** — while scanning, a line shows `Scanning 1234/5001
  (24.7%) | 3 in use | ETA 32s`. On a terminal it is rewritten in place; when
  output is piped it is printed to stderr so stdout stays clean for results.
- **Only in-use ports are printed** — idle ports are not listed and are not an
  error. Exit codes now reflect only real failures (permission denied, etc.).
- **Ports can span the whole range** `1-65535`; ports outside `1-65535` (e.g.
  `1-99999`) still fail fast before any work starts.
- Abort anytime with Ctrl-C.
- Standard Unix redirection (`portlens 3000-8000 > scan.txt` or `| tee scan.txt`)
  captures stdout cleanly because progress and ETA lines stream to stderr.

---

## Finding ports by process — `portlens find` (or `--pid` / `--name`)

Instead of guessing a port, start from the process:

```bash
$ portlens find 48231                # all ports owned by PID 48231 (incl. descendants)
$ portlens find python               # ports owned by processes matching "python"
$ portlens find "/next|vite/"        # regex match on name/command/exe
$ portlens find postgres --kill -y   # terminate matching processes
$ portlens --pid 48231               # flag equivalent
$ portlens --name python             # flag equivalent
```

**Use case — "which process owns these ports?"** A supervisor (like `node`, a
dev server, or a container runtime) often spawns children that actually bind.
`portlens find` matches process names, commands, executable paths, or PIDs
(including descendants). Wrap the query in `/.../` to use a regular expression.
The resulting ports behave exactly like a multi-port invocation, so every flag
(`--kill`, `--restart`, `--json`, ...) works.

---

## Watch mode — `--watch` and `--notify`

```bash
$ portlens 3000 --watch                    # re-render every second
$ portlens 3000 --watch --interval 2       # poll every 2 seconds
$ portlens --watch                         # watch the full listing
$ portlens 3000 --watch --notify           # notify on up/down/change
```

On a terminal, each tick redraws in place. Piped output reprints each snapshot
with a timestamp. Press `Ctrl-C` (or send `SIGTERM`) to exit cleanly.

**Use case — "is my server up yet?"** `portlens 3000 --watch` live-refreshes so
you can see the exact moment the process binds, starts serving, or crashes.
With `--notify`, macOS (`osascript`) and Linux (`notify-send`) post a desktop
notification the instant a watched port comes up, goes down, or changes owner —
no need to keep staring at the terminal.

---

## Named port groups — `portlens config`

Grouped ports save you from typing the same list repeatedly. Groups live in a
small JSON file (`portlens config path`), which on macOS is
`~/Library/Application Support/portlens/config.json`.

```bash
$ portlens config add dev 3000 4000-4010 5000
Saved group @dev: 3000, 4000-4010, 5000

$ portlens config list
Port groups (/Users/you/Library/Application Support/portlens/config.json):
  @dev   3000, 4000-4010, 5000

$ portlens config show dev
3000, 4000-4010, 5000

$ portlens config remove dev
$ portlens config path
/Users/you/Library/Application Support/portlens/config.json
```

Then reference a group anywhere a port is expected:

```bash
$ portlens @dev                 # inspect the whole group
$ portlens @dev --kill --yes    # stop them all
```

---

## Finding the next free port — `portlens next [start]`

```bash
$ portlens next                          # defaults to 3000
3000

$ portlens next 8000                     # find free port >= 8000
8002

$ PORT=$(portlens next 3000) npm start   # pipe directly into dev scripts
```

---

## Shell autocompletion — `portlens completion <shell>`

PortLens supports dynamic shell autocompletion that completes flags, subcommands,
and **currently listening ports** in real-time.

```bash
# Zsh (add to ~/.zshrc)
eval "$(portlens completion zsh)"

# Bash (add to ~/.bashrc)
eval "$(portlens completion bash)"

# Fish (add to ~/.config/fish/config.fish)
portlens completion fish | source
```

Once enabled, pressing `<TAB>` suggests active ports along with process names:

```bash
$ portlens 3<TAB>
3000  (node - next-app)    3001  (python - api)
```


