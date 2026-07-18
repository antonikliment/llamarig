# Glossary

Domain terms used across code, docs, and ADRs. Keep names consistent with this file.

- **Control daemon** — the `llamarig serve` process. Owns router orchestration and model presets and exposes the **ControlService RPC** on the **control socket**. No GUI, no TCP listener.
- **Web gateway** — the `llamarig gateway` process. Serves the web GUI, its app-owned `/api/*` REST facade, `/health`, `/info`, and `/mcp` on `listen_addr` (default `127.0.0.1:7000`). Calls the control daemon over the control socket; never calls `core/control` directly.
- **Control socket** — the Unix domain socket at `~/.llamarig/run/control.sock` (path from `config.ControlSocketPath()`) that the control daemon listens on. Every protocol adapter (CLI, TUI, web gateway, MCP) dials this socket via `rpc.DialControl` instead of duplicating the path or the dialer.
- **Entrypoint TUI** — the interactive dashboard launched by bare `llamarig` (or `llamarig tui`). The intended first/default surface for interactive use; it auto-starts the configured startup services on launch.
- **Startup services** — the `startup_services` config key (`control`, `web`, or both) chosen during setup. Distinct from `router.autostart_presets`, which lists model **Presets**, not LlamaRig services. The TUI reads this list on init and starts whichever configured service is not already running.
- **Router** — the single supervised llama-server process. It reads `$LLAMARIG_HOME/models.ini`, exposes the llama.cpp router endpoints on loopback, and owns loaded model processes.
- **Preset** — a named section in `$LLAMARIG_HOME/models.ini`. Loaded and unloaded through the Router; it is configuration, not an independently supervised process.
- **Unavailable Preset** — a **Preset** whose `model` file or `models-dir` directory cannot be used. It remains visible and editable, but LlamaRig blocks Start/Restart before calling the Router. Cleanup removes it and matching default/autostart references.
- **Local model** — a GGUF file in managed model storage (`model_storage_dir`, e.g. `~/.llamarig/models/{owner}/{repo}/{file}.gguf`).
- **Configured / Unconfigured model** — whether any **Preset** references a local model (via `model` or `models-dir`). The `used_by_presets` list names referencing presets. _Avoid_: "orphan model".
