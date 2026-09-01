# Security Policy

PortLens is a local-first developer utility. It reads process and network state
from the machine it runs on, keeps configuration on that machine,
and never transmits data anywhere.

## Reporting a vulnerability

PortLens is early-stage software and its security surface is small, but if you
believe you have found a security issue:

- **Do not open a public issue.**
- Report it privately using GitHub's
  [private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
  on the repository, so it can be triaged before it is disclosed.

Please include:

- A description of the issue and why it matters.
- Steps to reproduce, with a minimal example.
- The version or commit you tested.

## Scope and expectations

- We treat anything that could cause **unsafe process actions** (killing the
  wrong process, privilege escalation, bypassing confirmation) as high
  priority.
- Leaking user environment data (process command lines, working directories)
  outside the machine would also be considered a vulnerability.
- PortLens is not a sandbox: it reads process state via the OS and does not
  claim to be hardened against a malicious local actor who already has code
  execution.

## Security model

See [docs/security.md](docs/security.md) for the full threat model and design
decisions (no privilege escalation, graceful-then-force kill, facts vs.
inferences, local-only storage).
