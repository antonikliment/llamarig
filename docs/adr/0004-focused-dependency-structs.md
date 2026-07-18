# ADR 0004: Focused dependency structs

## Status

Accepted

## Context

ADR 0002 introduced a centralized `control.Services` bag to avoid constructor families while moving production wiring into `bootstrap`.

As HTTP, MCP, CLI, core control, app filesystem, auth, and audit wiring grew, the bag became cross-layer: core packages received protocol concerns and protocol adapters depended on core-only dependencies they did not use.

## Decision

Keep `bootstrap` as the production composition root and keep one exported constructor per production type, but use package-local dependency structs instead of one cross-layer bag.

`core/control` accepts `control.Dependencies`. Protocol adapters accept their own dependency structs. Bootstrap is responsible for assembling those structs and wiring concrete adapters.

## Consequences

- This supersedes the `control.Services` bag guidance in ADR 0002.
- The single-constructor rule remains.
- Core packages no longer receive HTTP-only settings.
- Protocol adapter constructors expose only adapter-facing dependencies.
