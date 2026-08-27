# Security & privacy model

PortLens is a **local-first** developer utility. It is designed so that using it
never leaks information about your machine.

## Guarantees

- **No telemetry.** There is no analytics, crash reporting, or phone-home of
  any kind.
- **No cloud service.** PortLens never opens a network connection to a
  third-party service as part of its operation. (The only network activity it
  can perform is the explicitly requested `--open`, which opens your local
  browser to a local URL.)
- **No data exfiltration.** Process names, commands, environment data, and
  working directories never leave your machine.
- **Local history only.** Optional port history is stored in a SQLite file under
  your OS data directory (`~/Library/Application Support/portlens` on macOS,
  `$XDG_DATA_HOME/portlens` or `~/.local/share/portlens` on Linux). Nothing is
  transmitted. Disable recording with `--no-record`.

## Secret handling

- Environment variable *values* are never printed.
- Only well-known launchd/systemd `KEY=value` argv artifacts (e.g.
  `XPC_FLAGS=1`, `LOGNAME=user`) are stripped from the human display; the raw
  `cmdline` remains available via `--json` for your own review.
- PortLens does not read or inspect environment variable values for any
  purpose.

## Destructive actions

- Kill and restart **always confirm** before acting unless you explicitly pass
  `--yes` or `--force`.
- The default kill is a graceful `SIGTERM`; `SIGKILL` is only sent via an
  explicit `--force`.
- Confirmation is refused when stdin is not a terminal and no bypass flag was
  given, so scripts cannot accidentally destroy a process.

## Privileges

- PortLens **never attempts to escalate privileges** (no `sudo`, no setuid).
- If inspecting or signaling a process requires elevated privileges, PortLens
  reports the permission problem and stops.

## Reporting

If you find a security issue, please report it responsibly by opening a private
disclosure channel with the maintainers (do not open a public issue for a
suspected vulnerability).
