# ADR 0011: Router-owned model presets

## Status

Accepted. Supersedes the profile-runtime portions of ADR 0006 and ADR 0007.

## Context

LlamaRig now supervises one llama.cpp router process. Named model configurations are sections in the root `$LLAMARIG_HOME/models.ini`; they are not independent YAML profiles or independently supervised processes.

Keeping profile-era config, command construction, status fields, and compatibility names made every adapter model two incompatible lifecycles.

## Decision

- **Preset** means one named `models.ini` section. `core/modelpresets` owns parsing, normalization, and durable persistence.
- **Router** means the single supervised llama-server process. `core/runtime` owns its process lifecycle and PID recovery; its PID file is `router.pid`.
- `config.router` owns router executable, port, capacity, default preset, autostart presets, timeouts, and environment.
- Runtime status lists loaded presets. Resource status reports the router OS process. These are distinct interfaces.
- Public RPC, REST, CLI, MCP, TUI, and web terminology uses preset/router without profile compatibility aliases.
- Router host and readiness path are fixed to loopback and `/health`. Idle unload is absent until request-aware behavior exists.

## Consequences

- `core/llamacmd`, profile-port allocation, and profile compatibility fields are removed.
- Existing profile-era config and wire clients must be updated; no automatic migration is provided.
- Router capacity defaults to one loaded preset and can be changed with `router.models_max`.
- The ordinary e2e suite exercises router load/unload through a local stub; live e2e still verifies real llama.cpp behavior.
