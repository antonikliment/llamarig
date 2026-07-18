# ADR 0002: Bootstrap Service and Single Constructor Rule

## Status

Accepted. Partially superseded by ADR 0004 for dependency struct shape.

## Context

`main.go` had become the production composition root for setup, config, runtime, stores, audit, control, HTTP, CLI, and MCP wiring. That made startup wiring noisy and encouraged constructor variants such as `NewServerWithAuth`.

## Decision

Production assembly belongs in `bootstrap`. The bootstrap package may import concrete protocol adapters and core implementations, build the production graph, and return the assembled service. Core and protocol packages must not import `bootstrap`.

A production type may have only one exported constructor. ADR 0004 supersedes the original centralized `control.Services` bag guidance with focused package-local dependency structs.

Tests may still construct dependencies directly with fakes or concrete stores. The bootstrap package is not a global service locator and must not expose package-level `Get` or `Resolve` helpers.

## Consequences

`main.go` owns process lifecycle and OS signals. `bootstrap` owns production dependency assembly and startup-time initialization such as configured runtime autostart.

`go test ./...` includes an architecture guard that fails when obvious constructor families are reintroduced.
