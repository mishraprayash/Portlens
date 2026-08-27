# Changelog

All notable changes to PortLens are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and this project adheres to
[Semantic Versioning](https://semver.org/).

## [Unreleased]

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
