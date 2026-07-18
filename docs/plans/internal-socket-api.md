# Internal Socket API Plan

## Checkpoint 1: Internal Socket API

Implemented scope:

- Daemon exposes the existing `/api/*` HTTP surface on a Unix socket at `$LLAMARIG_HOME/run/control.sock`.
- Internal socket API has no bearer auth or origin checks.
- Public HTTP server on `listen_addr` remains unchanged and still serves web UI assets, `/api/*`, `/mcp`, `/health`, and `/info`.
- Socket directory is created with restrictive permissions and the socket file is chmodded after bind.
- Bootstrap fails if an existing socket is active, but removes stale socket files before binding.
- Package ownership now separates `core/rpc` from adapters:
  - `core/rpc` owns the ControlService RPC schema, generated code, server, and Unix-socket transport.
  - `adapters/public_http` owns the TCP-facing public HTTP server wrapper.
  - `adapters/mcp`, `adapters/cli`, and `adapters/tui` own their adapter-specific client behavior.

## Follow-Up Plan

Completed in this branch:

1. Moved ControlService RPC server and mapping behavior into `core/rpc`.
2. Moved CLI, MCP, public HTTP, and TUI surfaces under `adapters/`.
3. Moved the ControlService protobuf schema and generated code under `core/rpc`.
4. Moved public HTTP wrapper behavior into `adapters/public_http`.

Remaining:

1. Keep adapters on ControlService RPC instead of calling `core/control` directly.
2. Keep public adapter auth and origin checks outside the control RPC socket.
3. Update this plan or replace it with an ADR if the socket contract changes again.

## Checkpoint Boundary

Stop here after ControlService RPC socket integration is implemented and tested.
