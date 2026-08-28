# Architecture

PortLens is a layered Go application. Platform-specific behavior is isolated in
build-tagged files; the rest of the codebase is OS-independent.

## Layers

```
┌─────────────────────────────────────────────────────────┐
│ cmd/                                                    │
│   argument parsing, dispatch, exit-code mapping, TUI loop│
├─────────────────────────────────────────────────────────┤
│ internal/render/        internal/actions/                │
│   terminal UI, tables,   kill / restart / open / copy     │
│   tree, JSON, history                                     │
├─────────────────────────────────────────────────────────┤
│ internal/inspector/                                      │
│   orchestrates providers → model.Report (+ risk)          │
├──────────────────────────┬──────────────────────────────┤
│ internal/detect/         │ internal/history/             │
│   project/runtime/       │   owner-only JSONL log        │
│   framework detection    │                               │
├──────────────────────────┴──────────────────────────────┤
│ internal/model/                                          │
│   shared OS-independent data types                        │
├─────────────────────────────────────────────────────────┤
│ internal/platform/  (the OS abstraction)                 │
│   PortResolver          ProcessInspector                 │
│   NetworkInspector      ProcessTreeProvider              │
│   ClipboardProvider     ProcessController                │
│   ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│   │ darwin_*.go  │  │ linux_*.go   │  │ windows_*.go │  │
│   │ sysctl+proc, │  │ /proc, xclip │  │ (planned)    │  │
│   │ lsof fallback│  │              │  │               │  │
│   └──────────────┘  └──────────────┘  └──────────────┘  │
└─────────────────────────────────────────────────────────┘
```

## The abstraction layer

`internal/platform/platform.go` declares the interfaces. Concrete
implementations live in files with build constraints:

| Interface             | macOS                                  | Linux                          |
|-----------------------|----------------------------------------|--------------------------------|
| `PortResolver`        | `lsof` (native lookup considered; see [performance.md](performance.md)) | parse `/proc/net/tcp{,6}` |
| `NetworkInspector`    | `lsof -a -p PID -i -F...`              | `/proc/<pid>/fd` → inode map   |
| `ProcessInspector`    | sysctl + libproc (native, no commands) | `/proc` (native, no commands)  |
| `ProcessTreeProvider` | sysctl `kern.proc.all` (one snapshot)  | one `/proc` scan                |
| `ClipboardProvider`   | `pbcopy`                               | `wl-copy`/`xclip`/`xsel`       |
| `ProcessController`   | `syscall.Kill`                         | `syscall.Kill`                 |

Process metadata is read natively on both platforms — `sysctl` and libproc
(`proc_pidpath`/`proc_pidinfo`) on macOS, `/proc` on Linux — with no external
commands. The process tree is built from a single native snapshot per
invocation. The only fast-path external command on macOS is `lsof` for port
resolution; see [performance.md](performance.md) for the security analysis of
replacing it.

The pure parsing helpers (`lsof.go`, `procnet.go`) live outside the build-tagged
files so they can be unit-tested on any platform.

## Data flow

1. `cmd` parses arguments and builds a `platform.Platform` and an
   `inspector.Inspector`.
2. `Inspector.Inspect(port)` calls `PortResolver.ResolvePort` to find listeners.
3. For the owning PID, it gathers `ProcessInfo`, ancestors, descendants, and
   connections via the providers.
4. `detect` infers project/runtime/framework from the working directory and
   command line.
5. The `model.Report` is produced, including `Facts` (observations) and
   `Inferences` (guesses), plus an exposure assessment.
6. `render` displays the report; `actions` performs any requested mutations.

## Design decisions

- **Facts vs. inferences** — the report keeps the two separate so callers can
  trust observations and treat guesses skeptically.
- **cgo-free** — builds with `CGO_ENABLED=0`, giving static binaries and easy
  cross-compilation.
- **Native-first, command fallback** — macOS reads process metadata natively
  via sysctl and libproc, and uses `lsof` only for port resolution (isolated
  behind the interface); Linux reads `/proc` directly with no external
  commands.
- **Deterministic JSON** — the JSON schema is stable and documented; see
  [json-schema.md](json-schema.md).
