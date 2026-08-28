# Performance

This document describes how PortLens stays fast, how it is measured, and where
the remaining costs live. The guiding rule is: **measure → identify → optimize →
benchmark → verify**. Never optimize on assumptions.

## Architecture

```
                  portlens
                      │
               Argument parsing
                      │
                      ▼
                 Query normalize
                      │
                      ▼
              Platform inspector
                      │
               ┌──────┴────────┐
               │   FAST PATH   │  portlens <port> (default, scan, multi-JSON)
               ▼               │
      Port → Socket → PID      │
               │               │
        Minimal metadata       │
        (name, exe, command,   │
        cwd, project, exposure,│
        service, origin,       │
        container)             │
               │               │
             Output            │
               │               ▼
              Exit        DEEP PATH
                          --verbose / --tree / --connections /
                          --json (single) / interactive [t] [n] /
                          --restart (needs the ancestor chain)
                          adds: process tree, network, facts
```

The fast path never pays the deep path's cost. Inspection depth is a single
flag (`inspector.DepthFast` vs `inspector.DepthFull`); interactive mode starts
fast and re-inspects with full depth only when the tree/connections keys are
pressed.

## Hot path

`main` → `Execute` → `parseArgs` → `runPorts` → `InspectDepth(Fast)` →
`ResolvePort` → `InfoBasic` → project/exposure/container → `Summary` → exit.

Design rules for the hot path:

- **No hidden external processes.** On Linux nothing is spawned at all. On
  macOS the single-port lookup shells out to `lsof` once (see Known
  bottlenecks). Process metadata is read natively — never via `ps`.
- **No process-table scans.** Children/Descendants/Ancestors are computed only
  at deep depth, from one native snapshot.
- **Lazy project/runtime detection** (walk-up from the working directory) runs
  only when an owning process is confirmed.
- **Container detection** fails fast: on Linux it reads `/proc/<pid>/cgroup`
  (a kernel fact); the Docker daemon socket is only dialed afterwards and a
  missing socket fails immediately.

## Cold (deep) path

`--verbose`, `--tree`, `--connections`, single-port `--json`, `--restart`, and
the interactive `[t]`/`[n]` keys opt into: the full ancestor chain, children,
the descendant tree, network connections, and verbose facts/inferences. All of
it is built from the **same single process-table snapshot** used for the tree,
so deep inspection is roughly `fast + one native table read + connections`.

## Native metadata (no gopsutil)

Process inspection is entirely native — there is no gopsutil dependency:

| Field | macOS | Linux |
|---|---|---|
| name, ppid, start, uid, zombie | `sysctl kern.proc.pid` (KinfoProc) | `/proc/<pid>/stat` |
| exe, cmdline | raw `__sysctl` KERN_PROCARGS2 syscall | `/proc/<pid>/cmdline`, `/proc/<pid>/exe` |
| cwd | libproc `proc_pidinfo` | `/proc/<pid>/cwd` |
| rss | libproc `proc_pidinfo` | `/proc/<pid>/statm` |
| process table | `sysctl kern.proc.all` (one call) | one `/proc` scan |

The only FFI is libproc on macOS, used strictly for cwd and RSS. The call
pattern is the one proven by gopsutil and re-validated here: register the
symbol via purego on the current OS thread immediately before the call, keep
the output buffer in the same stack frame, and read it before closing the
library. (Persistent trampolines and `SyscallN` were both found to return a
success code with an empty buffer in some scheduling cases; the per-call
pattern does not.)

## Benchmarks

Run `make bench`. Representative numbers (Apple M1, macOS, Go 1.23):

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkInspectPort` (deep, live port) | ≈32 ms | 259 KB | 1,104 |
| `BenchmarkInspectPortFast` (default, live port) | ≈15 ms | 87 KB | 450 |
| `BenchmarkParseProcNet` (300-line sample) | 1.3 µs | 2.4 KB | 16 |
| `BenchmarkDecodeAddr` | 131 ns | 16 B | 2 |
| `BenchmarkSummary` | 1.9 µs | 3.3 KB | 36 |
| `BenchmarkReportVerbose` | 8.5 µs | 11 KB | 100 |
| `BenchmarkJSONOutput` | 7.2 µs | 3.4 KB | 13 |
| `BenchmarkListTable` | 2.3 µs | 3.4 KB | 58 |
| `BenchmarkDarwinResolvePort` (lsof) | 15 ms | 52 KB | 166 |
| `BenchmarkDarwinListeners` (lsof ×1) | ≈15 ms | 60 KB | 180 |

## Before / after

Measured end-to-end on the same machine (`/usr/bin/time`, warm cache):

| Command | Before | After |
|---|---|---|
| `portlens 5432` (default) | ≈180–270 ms | ≈20–30 ms |
| `portlens 5432 --verbose` | ≈250 ms | ≈30 ms |
| `portlens` (listing) | ≈250 ms | ≈20 ms |
| `portlens --version` (fixed startup) | ≈10 ms | ≈10 ms |

Inspection micro-benchmarks (live port, owned by the benchmark process):

| Metric | Before (deep only) | After fast | After deep |
|---|---|---|---|
| latency | 72 ms | 15 ms | 32 ms |
| allocations | 17,742 | 450 | 1,104 |
| bytes | 3.6 MB | 87 KB | 259 KB |

### What changed and why

**Bottleneck #1 — default path paid for the deep path.**
- *Measurement:* `pprof` showed ≈85% of `Inspect` time in gopsutil
  `Children()`/`Descendants()`, which enumerate the whole process table per
  call; another chunk came from gopsutil `Info()` spawning `ps` twice on
  macOS.
- *Change:* `DepthFast` skips tree/network/verbose-facts; `InfoBasic` fetches
  only name/exe/cmdline/cwd/ppid (no `ps`).
- *Result:* default lookup ≈250ms → ≈20ms; allocations 16,700 → 450.

**Bottleneck #2 — deep path enumerated the process table repeatedly.**
- *Change:* a native process table (macOS: one `sysctl kern.proc.all` via
  `golang.org/x/sys/unix.SysctlKinfoProcSlice`; Linux: one `/proc` pass with a
  byte-oriented `/proc/<pid>/stat` parser) powers Ancestors/Children/
  Descendants from a single in-memory snapshot.
- *Result:* deep inspection ≈70ms → ≈32ms; allocations ≈15x lower.

**Bottleneck #3 — dependency weight and hidden `ps`.**
- *Change:* removed gopsutil and modernc/sqlite entirely. Process metadata is
  now native (`sysctl` + raw `__sysctl` + libproc on macOS; `/proc` on Linux);
  history is an owner-only JSONL log with atomic `O_APPEND` appends instead of
  an embedded SQLite database. Binary shrank **9.3 MB → 5.9 MB** and go.mod
  went from ~19 modules to 3 (purego, x/sys, x/term).
- *Change:* the macOS listing now issues **one** lsof call for both TCP LISTEN
  and UDP sockets (`-FpctnT`, with `TST=` distinguishing the protocol) instead
  of two.
- *Change:* `--restart` now re-runs the **raw argv directly** (no `sh -c`), so
  a crafted argv cannot inject shell syntax; it re-resolves the launch
  process's argv when the ancestor chain only carries identity.

**Bottleneck #4 — allocation churn in /proc parsing.**
- *Change:* byte-oriented `parseProcNet`/`decodeAddr` (manual hex/decimal
  parsing, no `fmt`, no per-token string conversions).
- *Result:* `BenchmarkParseProcNet` 25→16 allocs; `BenchmarkDecodeAddr` 4→2
  allocs.

## Syscall strategy

- **Linux port lookup:** read `/proc/net/{tcp,tcp6,udp,udp6}`, then map socket
  inodes to owners with one `/proc/<pid>/fd` scan per invocation (cached via
  `sync.Once`). Zero external processes.
- **macOS port lookup:** one `lsof` spawn (see Known bottlenecks).
- **Process tree:** one snapshot per invocation (sysctl or /proc); hierarchy
  queries are pure in-memory lookups afterwards.
- **Per-process metadata:** native syscalls only; the fast path (`InfoBasic`)
  never invokes `ps`.
- **Container detection:** `/proc/<pid>/cgroup` on Linux (a file read), then a
  unix-socket HTTP query to the Docker daemon only when a runtime is present.

## Allocation strategy

- Byte-oriented `/proc` parsing (no `fmt`, no regex) in `procnet.go` and the
  Linux process/table readers.
- `strings.Cut`-style parsing in `lsof.go`; the render layer streams to
  `io.Writer` instead of building large intermediate strings.
- `reportsToEntries` reuses the standard table renderer; JSON writes directly
  from typed structs (no `map[string]interface{}`, no reflection).
- Rendering is not allocation-free, but it is microseconds — inspection
  dominates and is where the budget is spent.

## Concurrency strategy

Intentionally **no goroutines** in the lookup paths. A single port needs one
`lsof`/`/proc` read and one metadata call; goroutine/channel overhead exceeds
the work. Deep inspection is sequential over the shared in-memory table. If
per-port inspection of *very large* scans ever becomes a bottleneck, bounded
worker pools would be evaluated — never unbounded per-process goroutines.

## Build profile

- `make build` — default (debuggable) build.
- `make build-release` — `-trimpath -ldflags="-s -w"`, stamped version; ≈5.9 MB
  static, no cgo.
- `make bench` / `make profile` — benchmark and profiling helpers.
- `go test -race ./...` — **blocked on macOS** by a Go-toolchain issue, not by
  project code: `-race` requires cgo, and Go 1.23.x + recent macOS produces
  cgo binaries missing the `LC_UUID` load command that `dyld` refuses to load
  (`missing LC_UUID load command`, reproduced on a dependency-free trivial
  package). This is the same documented issue that mandates `CGO_ENABLED=0`.
  CI runs `go test -race` on **Linux** (where cgo works); on macOS the full
  `CGO_ENABLED=0` suite covers correctness.

## Known bottlenecks

1. **macOS port lookup shells out to `lsof`** (≈15 ms; the listing uses one
   combined call ≈15 ms). Replacing it requires libproc
   (`proc_pidinfo`/`proc_pidfdinfo`), whose `struct socket_info` layout is
   internal to XNU, undocumented, and not stable across macOS versions — there
   is no maintained pure-Go reference. A layout drift would silently return a
   *wrong PID* for a port, and `--kill` would then signal the wrong process.
   That correctness/security risk is why the native resolver is deliberately
   deferred: the current path spawns exactly one external process and is
   correct. A native implementation with a size self-check, result validation,
   and an lsof fallback is the documented future optimization.
2. **libproc FFI on macOS** is the only non-syscall interface; it requires the
   register-on-thread call pattern described above and is exercised by
   repeated-call tests to guard against the empty-result failure mode.
3. **`decodeAddr` IPv6** still goes through `net.IP` (correct `::` compression)
   at 1 alloc; IPv4 is manual. Fine at current listener counts.

## Future optimizations

- Native macOS port→PID via libproc (bottleneck #1), keeping `lsof` as a
  correctness fallback.
- Parallelizing listener *rendering* (not inspection) for thousands of ports.
- A byte-level `/proc/net` port filter so `ResolvePort` parses only the rows
  matching the target port instead of the whole table.

## Budgets

Budgets are deliberately *relative* (regression tracking) rather than absolute
latency thresholds, which are hardware-dependent and flaky in CI. Watch these
via `make bench`:

- `BenchmarkInspectPortFast` should stay ≤ 2× its baseline (15 ms).
- `BenchmarkInspectPort` should stay ≤ 2× its baseline (32 ms).
- Default `portlens <port>` end-to-end should stay in the tens of
  milliseconds on warm cache.
