# Service and Package Ownership

This document defines where responsibilities belong. New logic should extend these owners instead of duplicating them.


| Responsibility | Owner |
|---|---|
| Config parsing, defaults, path resolution, control-socket path | `config/` |
| Router orchestration, loaded-preset snapshots, operation results, mutation locking, audit events | `core/control/` |
| llama-server router process lifecycle | `core/runtime/` |
| Root models.ini preset parsing and persistence | `core/modelpresets/` |
| llama.cpp router HTTP client | `core/router/` |
| Hugging Face model resolution/catalog | `core/modelcatalog/` |
| Model download jobs and target file safety | `core/modeldownload/` |
| Host/runtime telemetry | `core/signals/` |
| config.yaml persistence | `core/configstore/` |
| ControlService RPC schema, generated code, routing, error responses, control-client dialer | `core/rpc/` |
| First-run setup wizard/files | `core/setup/` |
| Public HTTP app serving, auth, origin checks, MCP mounting | `adapters/public_http/` |
| MCP tools/resources | `adapters/mcp/` |
| CLI protocol client commands | `adapters/cli/` |
| Terminal UI client | `adapters/tui/` |
| Embedded web UI assets | `webui/` |
| Service composition/wiring | `bootstrap/` |
| PID-file persistence and OS process identity | `platform/pidfile/` |
| Atomic file writes, backups, hashes, fsync | `platform/filedoc/` |
| Detached/child process execution | `platform/process/` |
| Active logs, log archives, retention, tailing, and Zap-backed audit sink | `platform/audit/` |

## Adapter Package Shape

The `adapters/` packages are concrete inbound adapters around core ownership.

- `core/rpc/` owns the Unix-socket ControlService RPC server, protobuf schema in `core/rpc/proto`, generated code in `core/rpc/gen`, protobuf mapping, and the shared `DialControl` client constructor used by every adapter. This is the core control API.
- `adapters/public_http/` owns the TCP-facing HTTP server wrapper: web UI assets, origin checks, bearer auth for public-only mounts, `/health`, `/info`, `/api/*`, and `/mcp`. Its app-owned `/api/*` routes adapt the internal ControlService RPC for the bundled web UI; they are not a stable third-party contract.
- `adapters/mcp/` owns MCP Streamable HTTP tools and resources. It calls ControlService RPC instead of `core/control` directly.
- `adapters/cli/` owns command-line client behavior. It calls ControlService RPC over the local Unix socket by default.
- `adapters/tui/` owns the terminal UI client, which also drives ControlService RPC.

Audit logging and file lifecycle live in `platform/audit/` as a cross-cutting
facility, not as an inbound adapter.

Adapter packages connect to `core/rpc` through ControlService RPC. Public adapter auth and origin checks stay outside `core/rpc`; the internal socket RPC is local-process control plumbing and intentionally has no bearer auth or origin guard.

Router process status, loaded-preset status, and operation results are separate
domain concepts. Store each in its owner and derive presentation fields at
protocol or UI boundaries.

## Rule for New Packages

A new package is allowed only when:

1. no existing owner fits,
2. the responsibility is added to this document,
3. the PR explains why extension was not enough.

## Frontend UI Ownership

Generic web UI lives in the copied shadcn-svelte modules under `webui/web/src/lib/components/ui`. Feature panels compose those modules directly with Tailwind utilities; LlamaRig does not maintain a second wrapper layer around buttons, cards, fields, status badges, lists, or dialogs.

Before adding generic UI, use the shadcn-svelte registry. Custom frontend modules are reserved for domain behavior absent from the registry, such as the YAML code editor and configuration diff rendering. Keep generated shadcn source near the registry version; place LlamaRig branding in semantic CSS variables and feature composition.
