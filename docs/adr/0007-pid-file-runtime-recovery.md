# ADR 0007: PID-File Runtime Recovery

## Status

Superseded by ADR 0011 for the single router process.

## Context

The core daemon can stop while managed `llama-server` processes continue
running. Recovery previously searched the host process table for an exact
command-line match. That search was platform-dependent and brittle when process
arguments or executable paths were represented differently by the operating
system.

LlamaRig already uses a PID file under `$LLAMARIG_HOME/run` for detached daemon
lifecycle state. Managed llama runtimes need the same deterministic ownership
record while still protecting against stale PID reuse.

## Decision

`core/runtime` writes one plain-text PID file per running profile at
`$LLAMARIG_HOME/run/llama/<profile>.pid`. The directory uses mode `0700`; PID
files use mode `0600`.

Each profile runtime owns exactly one `llama-server` process. Running multiple
instances means running multiple profiles; a profile does not batch an internal
process collection.

Recovery reads PID files only. A live PID is adopted when its executable
matches the configured llama executable. Readiness determines whether the
adopted process is reported as running or starting; lack of readiness does not
discard ownership. Missing PID files never trigger process-table discovery.

Malformed, dead, and executable-mismatched PID files are stale and removed.
Normal stop and observed process exit remove the matching PID file. Removal is
conditional on the recorded PID so an old process cannot delete a replacement
process's record.

Legacy `$LLAMARIG_HOME/run/runtime` JSON metadata is ignored and not migrated.

## Consequences

- Recovery is deterministic and independent of command-line enumeration.
- LlamaRig recovers only processes it previously started and recorded.
- PID reuse cannot cause LlamaRig to adopt or stop an executable with a different
  identity.
- Deleting a valid PID file intentionally gives up recovery ownership for that
  process.
