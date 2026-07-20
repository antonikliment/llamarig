# LlamaRig

[![Test](https://github.com/antonikliment/llamarig/actions/workflows/test.yml/badge.svg)](https://github.com/antonikliment/llamarig/actions/workflows/test.yml)
[![Lint](https://github.com/antonikliment/llamarig/actions/workflows/lint.yml/badge.svg)](https://github.com/antonikliment/llamarig/actions/workflows/lint.yml)
[![Release](https://img.shields.io/github/v/release/antonikliment/llamarig?include_prereleases)](https://github.com/antonikliment/llamarig/releases)

A local control plane for [llama.cpp](https://github.com/ggml-org/llama.cpp) server presets. Define named model presets once, then start, stop, and monitor them from whichever surface suits you: a TUI front door, a web GUI, an HTTP/MCP API, or a CLI client.

- start, stop, restart, and status for llama runtime presets
- create, read, replace, and delete `models.ini` presets
- browse and download GGUF models from Hugging Face, with fit estimates for your hardware
- system telemetry (RAM, CPU, NVIDIA GPU) in the TUI, web GUI, and API

## Prerequisites

- [llama.cpp](https://github.com/ggml-org/llama.cpp) built, with `llama-server` on your `PATH` (or note its full path — setup will ask for it). The [Docker setup](#docker) bundles `llama-server`, so it needs no separate llama.cpp install.

Optional: an NVIDIA GPU with `nvidia-smi` for GPU telemetry in the System tab/API.

Building from source additionally requires Go (see `go.mod` for the minimum version) and Node.js + [pnpm](https://pnpm.io/) (via `corepack`) to build the web GUI. Release binaries embed the web UI — no Node.js or pnpm needed.

## Install

Prereleases are published for Linux and macOS on amd64 and arm64 from the
[GitHub releases page](https://github.com/antonikliment/llamarig/releases). Pick
one of the methods below.

### Install script

The installer detects your OS/arch, downloads the matching release archive,
verifies its checksum, and installs the binary plus an `lr` shortcut. If `lr`
is already a command or occupies the install path, the installer leaves it
untouched. The installer can also set you up to run in Docker instead (it asks
when run interactively):

```bash
curl -fsSL https://raw.githubusercontent.com/antonikliment/llamarig/main/scripts/install.sh | sh

# non-interactive choices:
curl -fsSL .../install.sh | sh -s -- v0.1.0-alpha.2   # pin a release (binary)
curl -fsSL .../install.sh | sh -s -- --docker         # build and run via Docker
```

The same `install.sh` ships as an asset on each release.

### Manual release archive

Download the matching archive and `SHA256SUMS`, then verify before extracting:

```bash
# Linux
grep ' llamarig_<version>_linux_<arch>.tar.gz$' SHA256SUMS | sha256sum --check

# macOS
grep ' llamarig_<version>_darwin_<arch>.tar.gz$' SHA256SUMS | shasum -a 256 --check

tar -xzf llamarig_<version>_<os>_<arch>.tar.gz
install -m 0755 llamarig_<version>_<os>_<arch>/llamarig ~/.local/bin/llamarig
command -v lr >/dev/null 2>&1 || test -e ~/.local/bin/lr || test -L ~/.local/bin/lr || ln -s llamarig ~/.local/bin/lr
llamarig version
```

Release archives contain the LlamaRig binary, embedded web UI, README, and
license. `llama-server` remains a separate prerequisite and must be available on
`PATH` or selected during setup.

### Docker

The Docker image bundles `llama-server` (from the official
[`ghcr.io/ggml-org/llama.cpp`](https://github.com/ggml-org/llama.cpp/pkgs/container/llama.cpp)
image), so it is the only setup that needs no separately installed llama.cpp.
No prebuilt LlamaRig image is published yet, so Compose builds it locally:

```bash
docker compose up -d --build
# open http://127.0.0.1:7000/
```

Config, presets, and models persist in the `llamarig-home` volume mounted at
`/root/.llamarig`. On first boot the container writes a default `config.yaml`
and `models.ini` there; mount an existing `~/.llamarig` over that path to reuse
your own setup. The container binds to loopback (`127.0.0.1:7000`), so use
`http://127.0.0.1:7000` — not `localhost` — to satisfy the origin check.

**NVIDIA GPU (experimental).** Add the GPU override, which builds on the
`server-cuda` base and requests GPUs. It needs the
[NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html)
on the host; upstream ships the CUDA image but does not CI-test it, so treat it
as experimental:

```bash
docker compose -f docker-compose.yml -f docker-compose.gpu.yml up -d --build
```

**Windows (early / partial).** There are no native Windows binaries yet —
Windows process and local-control support are still in progress. Docker is the
current path on Windows: run the CPU setup above under Docker Desktop with the
WSL2 backend. GPU passthrough (WSL2 + NVIDIA Container Toolkit) is the
least-tested combination and should be considered experimental.

## Quickstart

Installed a release binary? Just run `llamarig`. Building from source, build the frontend once per checkout, then run LlamaRig:

```bash
corepack enable pnpm
cd webui && pnpm install && pnpm run build && cd ..
go run .
```

The first run (in an interactive terminal) walks you through a setup wizard: listen address, bearer token env var, `llama-server` path (checked against `PATH`), models directory, router port, and which services should start automatically (`control`, `web`, or both — see below). It writes `~/.llamarig/config.yaml` and `~/.llamarig/models.ini`.

`go run .` (or the built `llamarig` binary with no arguments) opens the **TUI** — the entrypoint for interactive use. The TUI auto-starts whichever services you configured in setup and shows a status notice while it does. From the TUI Services tab you can also start/stop services manually, and from the Models tab you can start the `default` preset once you've put a `.gguf` model in your models directory (or downloaded one through the web GUI).

Open the web GUI once the web gateway is running:

```text
http://127.0.0.1:7000/
```

For non-interactive startup (scripts, systemd, containers), create `~/.llamarig/config.yaml` first or set `LLAMARIG_CONFIG` — the wizard only runs in an interactive terminal.

## The two services

LlamaRig runs as two independent local processes:

| Service | Command | Transport | Purpose |
|---|---|---|---|
| **control daemon** | `llamarig serve` | Unix socket (`~/.llamarig/run/control.sock`) | Owns router orchestration, model presets, and the `ControlService` RPC that every other surface calls. No GUI, no TCP listener. |
| **web gateway** | `llamarig gateway` | TCP, `listen_addr` (default `127.0.0.1:7000`) | Serves the web GUI, the public `/api/*` REST facade, `/health`, `/info`, and `/mcp`. Talks to the control daemon over the same Unix socket. |

Both support `--detach`/`--foreground`, `down`, and `logs`:

```bash
llamarig serve --detach
llamarig down          # stop the detached control daemon
llamarig logs
llamarig logs --lines 500 --follow
llamarig logs --source gateway --lines 200
llamarig logs archive list
llamarig logs archive show <archive-id> --lines 500
llamarig logs archive delete <archive-id>
llamarig logs archive clear --yes

llamarig gateway --detach
llamarig gateway down
llamarig gateway logs
llamarig gateway logs --follow
```

When a detached service starts, any non-empty control and gateway logs are
archived under `~/.llamarig/logs/archive/`. Archives older than
`log_archive_retention` are removed at startup and every 24 hours; set it to
`0s` to retain archives indefinitely. The authenticated Logs page can tail,
pause, inspect, and delete archives without affecting active logs.

`startup_services` in `config.yaml` (set during setup, or edited directly) controls which of these the TUI starts automatically on launch — `control`, `web`, or both. Manual `llamarig serve` / `llamarig gateway` always work regardless of that setting.

## Entrypoints

- **TUI** — `llamarig` (no args) or `llamarig tui`. The recommended way to operate LlamaRig interactively: dashboard for both services, preset status, and system resources.
- **CLI client** — same binary, against a running control daemon:

  ```bash
  llamarig status
  llamarig presets
  llamarig preset default
  llamarig start default
  llamarig restart default
  llamarig stop default
  llamarig status --json
  ```

  Use `--socket` or `LLAMARIG_CONTROL_SOCKET` to target a non-default control socket.

- **HTTP / MCP** — see below, served by the web gateway.

## Security

This process can control a runtime and edit a config file. Treat it as trusted local tooling.

By default, no bearer token is configured and the service is intended for localhost use. If you bind the web gateway to `0.0.0.0`, `[::]`, `:7000`, or a hostname without configuring a token, LlamaRig still starts but logs a security warning. Set `security.auth_token_env` to an environment variable containing a bearer token before exposing the service.

For trusted LAN or reverse-proxy browser clients that cannot satisfy the same-origin check, set `security.disable_origin_check: true`. Do not expose this mode without bearer auth.

The control daemon's Unix socket intentionally has no bearer auth or origin guard; it is local-process control plumbing. Do not expose it outside the local host filesystem.

## Config

Config is loaded from `LLAMARIG_CONFIG`, then `~/.llamarig/config.yaml`.
Set `LLAMARIG_HOME` to use another LlamaRig home directory.

Default behavior:

- listen address: `127.0.0.1:7000`
- startup services: `control`, `web` (both)
- model presets: `~/.llamarig/models.ini`
- model storage dir: `~/.llamarig/models`
- Hugging Face catalog cache dir: `~/.llamarig/cache/hf-catalog`
- router PID file: `~/.llamarig/run/llama/router.pid`
- Hugging Face catalog cache TTL: `6h`
- router capacity: one loaded preset by default (`router.models_max`)
- autostart: presets listed in `router.autostart_presets`

See `config.example.yaml`.

LlamaRig uses these PID files to recover llama processes after the core daemon
restarts. It does not adopt externally started llama processes or read legacy
`~/.llamarig/run/runtime/**.json` recovery metadata.

To use a project-local config explicitly:

```bash
LLAMARIG_CONFIG=./config.yaml go run .
```

Each preset is a named section in `~/.llamarig/models.ini`. Keys use llama.cpp option names without leading dashes:

```ini
[default]
model = /path/to/model.gguf
n-gpu-layers = auto
flash-attn = on
ctx-size = 262144
ubatch-size = 2048
```

LlamaRig marks a preset unavailable when its local `model` file or `models-dir`
directory is missing. Start and restart are blocked before llama.cpp launches.
The web UI and TUI show the missing source and provide a confirmed cleanup
action that removes the preset and matching default/autostart references.

When Router is running, LlamaRig refreshes its model sources after every managed
Preset mutation. LlamaRig also watches `model_storage_dir` recursively and
refreshes after finalized `.gguf` files are added, changed, renamed, or removed.
Active Presets are restored when a source change unloads them. A stopped Router
stays stopped and reads fresh sources on its next start.

## HTTP

The public HTTP server (the web gateway) listens on `listen_addr` and serves the GUI, `/api/*`, `/mcp`, `/health`, and `/info`. Its `/api/*` routes are an app-owned REST facade over the internal `ControlService` RPC on the control daemon's Unix socket. They ship with the bundled UI and are not a stable third-party API; use the CLI or MCP for external automation.

```bash
curl http://127.0.0.1:7000/api/runtime/status
curl http://127.0.0.1:7000/api/signals
curl http://127.0.0.1:7000/api/events
curl http://127.0.0.1:7000/api/info
curl http://127.0.0.1:7000/api/health
curl -X POST 'http://127.0.0.1:7000/api/runtime/start?preset=default'
curl -X POST 'http://127.0.0.1:7000/api/runtime/stop?preset=default'
curl -X POST 'http://127.0.0.1:7000/api/runtime/restart?preset=default'
curl http://127.0.0.1:7000/api/presets
curl -X POST -H 'Content-Type: application/json' --data @preset.json http://127.0.0.1:7000/api/presets
curl http://127.0.0.1:7000/api/presets/default
curl -X DELETE http://127.0.0.1:7000/api/presets/default
curl -X POST -H 'Content-Type: application/json' \
  --data '{"url":"https://huggingface.co/unsloth/Qwen3.6-27B-MTP-GGUF"}' \
  http://127.0.0.1:7000/api/models/resolve
curl 'http://127.0.0.1:7000/api/models/catalog?limit=50&sort=downloads&min_fit=fits'
curl -N http://127.0.0.1:7000/api/models/catalog/events
curl -X POST -H 'Content-Type: application/json' \
  --data '{"url":"https://huggingface.co/unsloth/Qwen3.6-27B-MTP-GGUF","filename":"Qwen3.6-27B-UD-Q4_K_XL.gguf"}' \
  http://127.0.0.1:7000/api/models/downloads
curl http://127.0.0.1:7000/api/models/downloads/DOWNLOAD_ID
curl -X POST -H 'Content-Type: application/json' \
  --data '{"preset":"default","preview":true}' \
  http://127.0.0.1:7000/api/models/downloads/DOWNLOAD_ID/apply-to-preset
```

Model downloads support public Hugging Face model URLs in the form `https://huggingface.co/{owner}/{repo}`. LlamaRig stores selected `.gguf` files under `model_storage_dir/{owner}/{repo}/` and can apply a completed download by updating the preset's `model` entry.

The catalog endpoint lists ranked Hugging Face GGUF repos for the local machine. It serves cached catalog data immediately when present, refreshes stale cache entries in the background, and emits Server-Sent Events when refreshes complete.

Runtime signals expose current RAM, CPU, best-effort NVIDIA GPU telemetry when `nvidia-smi` is available, and resource usage for LlamaRig-managed runtime processes. The Models UI uses the same signal data for fit summaries and shows Hugging Face metadata tags as model chips with links back to the source repo.

The event timeline records recent mutating orchestration actions in memory.

## MCP

MCP Streamable HTTP endpoint, served by the web gateway:

```text
http://127.0.0.1:7000/mcp
```

Tools:

- `llama_info`
- `llama_status`
- `llama_start`
- `llama_stop`
- `llama_restart`
- `presets_list`
- `preset_get`
- `preset_put`
- `preset_delete`

## Development

During web GUI development, run Vite directly:

```bash
cd webui
pnpm run dev
```

To serve a local built UI with the Go process, point `LLAMARIG_APP_DIR` at the generated app directory:

```bash
cd webui
pnpm run build
cd ..
LLAMARIG_APP_DIR=./webui/dist llamarig gateway --foreground
```
