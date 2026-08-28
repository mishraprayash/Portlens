# JSON schema

`portlens <port> --json` emits a single JSON object on stdout. The schema is
deterministic and designed for shell scripts and AI agents.

## Top level

```jsonc
{
  "port": 3000,            // int32 — the requested port
  "protocol": "tcp",       // "tcp" | "udp"
  "status": "listening",   // "listening" | "not_listening"
  "address": "127.0.0.1",  // bind address of the primary listener
  "service": "PostgreSQL", // omitempty — well-known service name for the port
  "process": { ... },      // omitempty — owning process
  "origin": "user",        // omitempty — "system" | "user" (heuristic)
  "container": { ... },    // omitempty — owning container (docker), if any
  "ancestors": [ ... ],    // omitempty — chain, oldest first
  "children": [ ... ],     // omitempty — direct children
  "project": { ... },      // omitempty — detected project metadata
  "network": { ... },      // omitempty — listeners + connections + summary
  "exposure": { ... },     // omitempty — risk assessment
  "interpretation": "...", // human-readable one-liner
  "facts": [ ... ],        // concrete observations
  "inferences": [ ... ]    // best-effort guesses (do not treat as fact)
}
```

## `process` / ancestor / child objects

```jsonc
{
  "pid": 48231,
  "ppid": 4021,
  "name": "node",
  "exe": "/opt/homebrew/bin/node",
  "command": "pnpm dev",
  "cmdline": ["node", "pnpm", "dev"],
  "cwd": "/Users/example/projects/orbit/backend",
  "user": "example",
  "started_at": "2026-08-27T21:42:13.123+05:45",
  "cpu_percent": 0.1,
  "memory_bytes": 123456,
  "is_zombie": false,
  "is_target": true
}
```

## `container`

```jsonc
{
  "id": "8dfafdbc3a40b8b4f7c4a6f5e6b1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9",
  "name": "api-1",
  "image": "nginx:alpine",
  "status": "running",
  "compose_project": "orbit",
  "compose_service": "api"
}
```

`id` may be a short ID when only a cgroup-derived ID is available and the
daemon is unreachable. All fields are facts reported by the container runtime
(never guesses). Absent when the port is not owned by a container.

## `project`

```jsonc
{
  "name": "orbit-backend",
  "directory": "/Users/example/projects/orbit/backend",
  "git_repo": "orbit-backend",
  "git_branch": "main",
  "runtime": "node",
  "framework": "nestjs",
  "package_manager": "pnpm",
  "language": "javascript",
  "detected": true
}
```

## `network`

```jsonc
{
  "listeners": [
    { "protocol": "tcp", "address": "127.0.0.1", "port": 3000,
      "state": "LISTEN", "pid": 48231, "process": "node" }
  ],
  "connections": [
    { "pid": 48231, "protocol": "tcp",
      "local_address": "127.0.0.1", "local_port": 3000,
      "remote_address": "127.0.0.1", "remote_port": 52000,
      "state": "ESTABLISHED" }
  ],
  "summary": {
    "total": 8,
    "by_state": { "ESTABLISHED": 5, "TIME_WAIT": 3 },
    "local_only": true,
    "remote_count": 0
  }
}
```

## `exposure`

```jsonc
{
  "bound_localhost": true,
  "bound_wildcard": false,
  "public_interface": false,
  "findings": [
    { "level": "LOW RISK", "reason": "Bound only to loopback; not reachable from other machines" }
  ],
  "worst_level": "LOW RISK"
}
```

`level` and `worst_level` use the strings `"LOW RISK"`, `"WARNING"`, and
`"POTENTIALLY DANGEROUS"`.

## Listing entries (`portlens --json` with no port)

`portlens --json` (no port) emits an array of rows:

```jsonc
{
  "port": 5432,
  "protocol": "tcp",
  "process": "postgres",     // omitempty
  "pid": 946,                // omitempty
  "service": "PostgreSQL",   // omitempty — well-known service name
  "project": "brew",         // omitempty
  "runtime": "postgres",     // omitempty
  "address": "[::]",
  "status": "LISTEN",
  "origin": "user",          // omitempty — "system" | "user" (heuristic)
  "container": { ... }       // omitempty
}
```

## Stability notes

- Field names and types are stable within a major version.
- Omitted (`omitempty`) fields are absent, not `null`.
- Timestamps are RFC 3339 (with sub-second precision when available).
- `is_target` marks the process that owns the inspected port.
- `service` is a curated well-known-port lookup (a fact about the port, not a
  claim about the process).
- `origin` is a heuristic inference (executable path + process name); treat it
  as a hint, not a guarantee.
- `inferences` are explicitly not guaranteed — treat them as hints.
