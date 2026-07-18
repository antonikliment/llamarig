# ADR 0001: Core actions behind protocol adapters

## Status

Accepted. Partially superseded by ADR 0003, which was later superseded when managed profile `models.ini` was removed.

## Context

The project exposes the same llama runtime and profile operations over HTTP, MCP, and later a GUI.

## Decision

HTTP, MCP, CLI, and audit integrations are protocol adapters. They call shared use-case modules instead of owning runtime or profile persistence. Runtime management and profile persistence live behind interfaces.

`core/runtime` owns llama runtime interfaces and implementations. `core/control` coordinates those modules behind protocol adapters. `core/rpc` owns the shared internal `/api/*` HTTP handler. Public TCP HTTP concerns live in `adapters/public_http`.

## Consequences

- Runtime and file logic is tested once through the core interface.
- Auth can be added at the protocol seam.
- The GUI can stay thin and use the HTTP surface.
- New runtime adapters can be added without changing protocol handlers.
