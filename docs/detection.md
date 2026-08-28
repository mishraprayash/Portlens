# Detection philosophy

PortLens distinguishes **facts** from **inferences** and never presents a guess
as a guarantee.

## Facts

Things observed directly from the OS:

- PID, process name, parent PID, user, start time.
- The full command line (raw `argv`).
- The working directory.
- Listening sockets and their bind addresses.
- Active network connections and their states.
- **Service** — the well-known name registered for the port (a conservative,
  curated registry: IANA-registered services and widely deployed
  infrastructure, e.g. `5432 → PostgreSQL`, `5353 → mDNS (DNS-SD)`). This
  names what the port *is for*; it never claims what a process is doing.

These populate the `facts` array and the concrete report fields.

## Inferences

Things PortLens guessed, always labeled "best-effort, not guaranteed":

- **Runtime** — inferred from the process name/executable (`node`, `python`,
  `go`, `java`, `docker`, `postgres`, `redis`, ...).
- **Framework** — inferred from `package.json` dependencies (NestJS, Next.js,
  Express, Fastify, Prisma, ...) and/or command-line hints (`nest start`,
  `uvicorn`, ...).
- **Project** — the nearest directory (walking up from the working directory)
  containing a recognized marker (`.git`, `package.json`, `go.mod`,
  `pyproject.toml`, `Cargo.toml`, ...).
- **Git repo/branch** — read from `.git` metadata.
- **Origin** — whether the owning process is a **system** component (bundled
  with the OS, e.g. `kdc`, `mDNSResponder`, `rapportd`) or **user**-installed
  (Homebrew services, `/Applications`, language toolchains). Decided from the
  executable path when available, falling back to well-known system/third-party
  daemon names when the path can't be resolved (e.g. a process owned by another
  user). Shown as `system` / `user` in the listing, summary, and JSON; `-`
  when unknown.
- **Interpretation** — a one-line human summary ("NestJS process (Node.js)").

These populate the `inferences` array.

## What PortLens deliberately avoids

- **Claiming safety.** Exposure is reported as `LOW RISK` / `WARNING` /
  `POTENTIALLY DANGEROUS` with a reason; PortLens never says a service is
  "safe".
- **Printing secrets.** Environment variable *values* are never shown. Only
  well-known launchd/systemd `KEY=value` argv artifacts are stripped from the
  *display* command line; the raw command line remains available via `--json`
  for the user's own inspection.
- **Guessing restarts.** If the launch command cannot be confidently determined
  (e.g. a daemon started by launchd/systemd), `--restart` reports that automatic
  restart is unavailable instead of guessing.
