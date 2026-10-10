# Exit codes

PortLens exits with a meaningful code so shell scripts can branch on outcomes.

| Code | Constant              | Meaning                                           |
|------|-----------------------|---------------------------------------------------|
| 0    | `Success`             | Command completed successfully.                    |
| 1    | `GeneralError`        | Unexpected failure.                                |
| 2    | `InvalidArguments`    | Malformed flags or an invalid port value.          |
| 3    | `PortNotFound`        | Nothing is listening on the requested port.        |
| 4    | `PermissionDenied`    | The operation requires privileges PortLens will not obtain. |
| 5    | `ProcessActionFailed` | A process action (kill/restart) did not complete.  |

## Examples

```bash
portlens 3000 && echo "up"          # exit 0 when listening
portlens 9999; echo $?              # 3 when nothing is listening

if ! portlens 3000 --kill --yes; then
  case $? in
    4) echo "need sudo to kill that process";;
    5) echo "process did not exit; consider --force";;
  esac
fi
```

Notes:

- `portlens <port> --json` on a missing port still emits a JSON object with
  `"status": "not_listening"` and exits `3`, so scripts can consume JSON and the
  exit code together.
- A `--kill` that is refused because stdin is not a terminal (and no `--yes` /
  `--force` was given) is reported as an error; use `--yes` to script it.
- **Scan mode (several ports/ranges) treats "not listening" as normal**, not
  as exit `3`: idle ports are skipped and the command exits `0`. Ports that
  could not be inspected print `portlens: warning: N of M ports could not be
  inspected: ...` on stderr and also exit `0`; only a fatal error (bad
  arguments, permission problems aborting the run, Ctrl-C) sets a non-zero
  code.
