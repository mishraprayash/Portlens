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
                          --json (single) / interactive [t] [n]
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
  macOS the single-port lookup still shells out to `lsof` once (see Known
  bottlenecks). `InfoBasic` deliberately avoids gopsutil's `CreateTime` and
  `Status`, which spawn `ps` on macOS.
- **No process-table scans.** Children/Descendants/Ancestors are computed only
  at deep depth, from one native snapshot.
- **Lazy project/runtime detection** (walk-up from the working directory) runs
  only when an owning process is confirmed.
- **Container detection** fails fast: on Linux it reads `/proc/<pid>/cgroup`
  (a kernel fact); the Docker daemon socket is only dialed afterwards and a
  missing socket fails immediately.

## Cold (deep) path

`--verbose`, `--tree`, `--connections`, single-port `--json`, and the
interactive `[t]`/`[n]` keys opt into: the full ancestor chain, children, the
descendant tree, network connections, and verbose facts/inferences. All of it
is built from the **same single process-table snapshot** used for the tree, so
deep inspection is roughly `fast + one native table read + connections`.

## Benchmarks

Run `make bench`. Representative numbers (Apple M1, macOS, Go 1.23):

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkInspectPort` (deep, live port) | 32 ms | 259 KB | 1,104 |
| `BenchmarkInspectPortFast` (default, live port) | 15 ms | 87 KB | 450 |
| `BenchmarkParseProcNet` (300-line sample) | 1.6 µs | 1.8 KB | 25 |
| `BenchmarkDecodeAddr` | 158 ns | 32 B | 4 |
| `BenchmarkSummary` | 1.9 µs | 3.3 KB | 36 |
| `BenchmarkReportVerbose` | 8.5 µs | 11 KB | 100 |
| `BenchmarkJSONOutput` | 7.2 µs | 3.4 KB | 13 |
| `BenchmarkListTable` | 2.3 µs | 3.4 KB | 58 |
| `BenchmarkDarwinResolvePort` (lsof) | 15 ms | 52 KB | 166 |
| `BenchmarkDarwinListeners` (lsof ×2) | 28 ms | 120 KB | 359 |

## Before / after

Measured end-to-end on the same machine (`/usr/bin/time`, warm cache):

| Command | Before | After |
|---|---|---|
| `portlens 5432` (default) | ≈180–270 ms | ≈20–30 ms |
| `portlens 5432 --verbose` | ≈250 ms | ≈40 ms |
| `portlens` (listing) | ≈250 ms | ≈40 ms |
| `portlens --version` (fixed startup) | ≈10 ms | ≈10 ms |

Inspection micro-benchmarks (live port, owned by the benchmark process):

| Metric | Before (deep only) | After fast | After deep |
|---|---|---|---|
| latency | 72 ms | 15 ms | 32 ms |
| allocations | 17,742 | 450 | 1,104 |
| bytes | 3.6 MB | 87 KB | 259 KB |

### What changed and why

**Bottleneck #1 — default path paid for the deep path.**
- *Measurement:* `pprof` showed ≈85% of `Inspect` time in `Children()` /
  `Descendants()`, which enumerate the whole process table per call via
  gopsutil; another chunk came from gopsutil `Info()` spawning `ps` twice on
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

## Syscall strategy

- **Linux port lookup:** read `/proc/net/{tcp,tcp6,udp,udp6}`, then map socket
  inodes to owners with one `/proc/<pid>/fd` scan per invocation (cached via
  `sync.Once`). Zero external processes.
- **macOS port lookup:** one `lsof` spawn (see Known bottlenecks).
- **Process tree:** one snapshot per invocation (sysctl or /proc); hierarchy
  queries are pure in-memory lookups afterwards.
- **Per-process metadata:** gopsutil over native syscalls; the fast path calls
  the minimum fields and never invokes `ps`.
- **Container detection:** `/proc/<pid>/cgroup` on Linux (a file read), then a
  unix-socket HTTP query to the Docker daemon only when a runtime is present.

## Allocation strategy

- Byte-oriented `/proc` parsing (no `fmt`, no regex) in `proctable_linux.go`;
  reusable buffers where practical.
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
- `make build-release` — `-trimpath -ldflags="-s -w"`, stamped version; ≈9.3 MB
  static, no cgo.
- `make bench` / `make profile` — benchmark and profiling helpers.
- `go test -race ./...` — **blocked on macOS** by a pre-existing gopsutil /
  ebitengine-purego incompatibility with the Go race runtime: any binary that
  imports `internal/platform` segfaults during race-runtime init
  (`signal arrived during cgo execution`, addr=0x10), reproduced on the
  baseline commit before the performance work. CI therefore does not run
  `-race`; correctness is covered by the full test suite and
  `CGO_ENABLED=0` (which the race detector cannot use anyway).

## Known bottlenecks

1. **macOS port lookup shells out to `lsof`** (~15ms; ~28ms for the full
   listing). Replacing it requires libproc (`proc_listallpids` +
   `proc_pidinfo`/`proc_pidfdinfo`), which needs purego FFI with a large,
   version-sensitive `struct socket_info` replication. No maintained pure-Go
   reference exists in the dependency tree; hand-rolling it risks silent
   wrong answers on some macOS versions. A native lookup with an lsof
   fallback is the documented future optimization; the current default path
   still spawns exactly one external process.
2. **`modernc.org/sqlite` (history) + gopsutil are the binary-size drivers**
   (≈9.3 MB total). History is opened lazily (never on `--no-record`), and
   gopsutil is used only for per-process metadata; both are justified by
   features but are the first candidates if size ever matters more.
3. **`decodeAddr`** allocates 4×158ns per address on Linux; fine at current
   listener counts, a candidate for a byte-based IP formatter if /proc
   parsing ever shows up on a profile.

## Future optimizations

- Native macOS port→PID via libproc (bottleneck #1), keeping `lsof` as a
  correctness fallback.
- Parallelizing listener *rendering* (not inspection) for thousands of ports.
- A byte-level `/proc/net` port filter so `ResolvePort` parses only the rows
  matching the target port instead of the whole table.
- `go vet`-clean replacement of gopsutil's darwin metadata with native
  `proc_pidpath`/argv/cwd reads to drop the remaining purego dependency and
  unblock `-race` on macOS.

## Budgets

Budgets are deliberately *relative* (regression tracking) rather than absolute
latency thresholds, which are hardware-dependent and flaky in CI. Watch these
via `make bench`:

- `BenchmarkInspectPortFast` should stay ≤ 2× its baseline (15 ms).
- `BenchmarkInspectPort` should stay ≤ 2× its baseline (32 ms).
- Default `portlens <port>` end-to-end should stay in the tens of
  milliseconds on warm cache.
