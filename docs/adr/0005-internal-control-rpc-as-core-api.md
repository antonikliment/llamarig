# ADR 0005: Internal Control RPC as Core API

## Status

Accepted. Its public REST compatibility consequence is superseded by ADR 0015.

## Context

The internal socket originally exposed HTTP `/api/*` handlers. Public HTTP, MCP, and CLI could still call `core/control` or the REST surface directly, so protocol adapters duplicated routing and response behavior.

## Decision

`core/rpc` owns the internal ControlService RPC server on the Unix socket. Protocol adapters connect to that RPC interface:

- public HTTP translates existing REST `/api/*` routes to ControlService RPC calls,
- MCP tools and resources call ControlService RPC,
- CLI commands call ControlService RPC over the local socket by default.

Public auth and origin checks remain in `adapters/public_http`. The internal socket remains unauthenticated local control plumbing.

## Consequences

- Core control behavior has one RPC interface for protocol adapters.
- Public REST stays at the public HTTP adapter; ADR 0015 defines it as app-owned rather than a third-party compatibility contract.
- CLI is local-socket oriented by default instead of remote public REST oriented.
- New protocol adapters should use ControlService RPC, not `core/control` directly.
