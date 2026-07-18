# ADR 0010: Layered top-level package layout

## Status

Accepted.

## Context

The top-level package layout had drifted into vague, overloaded names:

- `core/` read as a generic umbrella.
- The adapter directory held MCP, HTTP, CLI, and TUI behind a name that implied
  every entry was a transport protocol, even though CLI and TUI are user
  interfaces.
- The Go import-protection directory had become a flat grab-bag: generated
  protobuf, the composition root, OS plumbing, an audit sink, and the first-run
  wizard all together. In a single-binary module that protection bought little,
  so the directory was effectively being used as a semantic label.

We want each top-level directory to announce its architectural layer.

## Decision

Adopt a layer-based top-level layout:

- `cmd/` — entrypoints (unchanged).
- `config/` — config parsing/defaults/paths (unchanged).
- `core/` — domain logic and the control RPC API. Absorbs the first-run wizard
  as `core/setup` and owns the ControlService schema and generated protobuf
  code under `core/rpc/proto` and `core/rpc/gen`.
- `adapters/` — inbound adapters: `mcp`, `public_http`, `cli`, `tui`. Matches
  the "protocol adapters" language already used in ADR 0001 and ADR 0005.
- `platform/` — OS and cross-cutting plumbing: `pidfile`, `filedoc`, `process`,
  `audit`.
- `bootstrap/` — composition root, lifted to the top level because it sits
  above every layer (it imports `core`, `adapters`, `platform`, and `webui`).
- `webui/` — the embedded web frontend.

Package identifiers were preserved except `webui`; the change was almost
entirely import-path string updates plus the ControlService contract move under
`core/rpc`.

## Consequences

- Every top-level directory now names its layer.
- The ControlService contract sits with the module that owns the RPC interface,
  so the schema and generated client/server types share `core/rpc` locality.
- The living package ownership map is `docs/architecture/services.md`.
