# ADR 0015: App-owned public REST

## Status

Accepted.

## Context

The public gateway's `/api/*` routes were described as compatibility REST, but the only supported consumer is the web app embedded in the same binary. The TUI, CLI, and MCP adapters use the internal ControlService RPC or their own protocol. Keeping routes after the app stops using them adds code, tests, and exposed operations without preserving a supported integration contract.

## Decision

The public `/api/*` REST facade is app-owned, not a stable third-party API. A route may be removed with its bundled web consumer when no other supported in-repository consumer remains. External automation should use the CLI or MCP.

The ControlService RPC on the Unix socket remains the internal contract between protocol adapters. The TUI and web gateway share `GetSignals` rather than maintaining separate telemetry projections.

The unused `/api/runtime/resources` and `/api/config.yaml*` routes are removed. The deprecated apply-download `restart` field is removed; callers explicitly start a runtime when desired, while the existing automatic Router source refresh remains unchanged.

## Consequences

- The public gateway exposes only routes needed by its bundled app and gateway-owned features.
- Raw HTTP clients using the removed routes must migrate to the CLI, MCP, or an internal socket client.
- The bundled app and its REST facade continue to ship together, so they need no compatibility window between versions.
- The bundled web UI now uses the existing ControlService through a public
  Connect RPC proxy. Only gateway-owned health/info, logs, and MCP HTTP routes
  remain outside that schema.
- This supersedes the public REST compatibility consequence in ADR 0005.
