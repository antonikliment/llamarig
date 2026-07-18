# ADR 0006: Runtime Snapshot and Operation State

## Status

Partially superseded by ADR 0011.

## Context

LlamaRig exposes command previews, runtime status, and mutation outcomes through HTTP,
MCP, CLI, RPC, and the web UI. These are related but separate concepts:

- command preview: the deterministic `llama-server` command for a profile,
- runtime snapshot: the current state of managed profile runtimes,
- operation result: the lifecycle outcome of a mutating action.

The previous implementation duplicated these concepts across core types,
protobuf messages, protocol maps, and frontend state. Presentation fields such
as active badges and health labels were also stored as state in some layers.

## Decision

`core/llamacmd` owns deterministic command construction. `core/serverconfigs`
owns profile file access and returns command previews by reusing the canonical
command type.

`core/control` owns profile runtime slots, runtime snapshot aggregation, active
profile derivation, and operation lifecycle results. `core/runtime` remains the
owner of `llama-server` process lifecycle, readiness probing, and process-level
status.

`core/rpc` maps core control data to ControlService protobuf messages and RPC
errors. Protocol packages may format or transport responses, but must not
recreate business state models. Derived presentation labels belong at protocol
or UI boundaries and must not become stored domain state.

Deprecated compatibility fields and aliases should be removed once internal
consumers have migrated. If compatibility layers push Go implementation LOC
over the active lint budget, remove the compatibility layer rather than
duplicating logic elsewhere.

## Consequences

- Runtime state has one manager-owned source of truth.
- Operation responses use first-class target, status, message, and error kind.
- Command preview has one Go field definition.
- Protocol and frontend code derive presentation state from snapshots.
- New protocol adapters should use ControlService RPC and shared mapping
  helpers instead of calling `core/control` directly or rebuilding DTOs.
