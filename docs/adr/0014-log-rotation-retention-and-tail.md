# ADR 0014: Log rotation, retention, and tailing

## Status

Accepted

## Decision

`platform/audit` owns active control/gateway log paths, restart rotation, archive access, bounded tailing, and retention cleanup.

Every detached start moves a non-empty active log to `~/.llamarig/logs/archive/` with a UTC timestamp before opening a new active file. Archives remain plain text. `log_archive_retention` defaults to `168h`; `0s` disables automatic deletion. Each long-running service cleans archives at startup and every 24 hours.

CLI and authenticated HTTP adapters call the shared audit operations directly. Archive identifiers are validated basenames; symlinks and unrelated files are ignored. Archive deletion never touches active logs.

## Consequences

- Restarts no longer mix separate process sessions in one file.
- CLI and web UI can tail current or archived logs without loading whole files.
- Control and gateway may run cleanup concurrently; deletion is idempotent.
- Size-based rotation and compression remain out of scope.
