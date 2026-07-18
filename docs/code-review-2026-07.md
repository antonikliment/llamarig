# LlamaRig Code Review — July 2026

Deep review of all hand-written Go (101 files, ~11.4k LOC, excluding `core/rpc/gen/`)
and the 7 `.proto` files. Focus: dead code, LOC reduction, improvements.
Claims below were verified by grep against the working tree at branch `ux-two` (5ede8f3).

## Implementation status — 18 July 2026

The approved safe-core and app-owned REST cleanup is implemented on `dev-code-review` in commits `6c26aa8` and `f7d5b55`.

- Removed the confirmed dead Go, proto, REST, and Svelte surfaces, including `/api/runtime/resources`, `/api/config.yaml*`, the unused CodeMirror editor, and the deprecated apply-download `restart` option. Internal runtime-resource and config RPC behavior remains available to current consumers.
- Fixed detached-start error handling and unlocked help-cache misses. Startup-service YAML edits now reuse `configstore`'s validated document mutation.
- Reused a signals-owned machine collector for catalog fit data, selected the largest GPU by total VRAM, surfaced partial catalog failures, and recovered from corrupt catalog caches through a fresh fetch.
- Corrected Svelte operation/download state styling and exposed partial catalog errors without hiding successful results. Focused Go tests and the complete Svelte check, test, architecture, and production-build pipeline cover the changes.
- Kept the meaningful runtime `Stopping` and `Failed` states. The larger Connect proxy/code-generation migration in checkpoint 6 remains deferred; ADR 0015 records `/api/*` as an app-owned facade instead of a third-party compatibility contract.
- Hand-written implementation Go is 9,978 gocloc-counted lines, below the 10,025-line quality budget.

## Checkpoint 1 — Dead code

### Go

| Location | What | Est. LOC |
|---|---|---|
| `adapters/mcp/transport.go` (whole file) + `Dependencies.Manager` | `newInProcessControlClient` is only reached when `Dependencies.ControlClient` is nil; the sole caller (`public_http/routes.go:64`) always passes a client. Delete file, the nil-fallback in `server.go`, and the `Manager` field (and its `control` import). | ~35 |
| `core/control/events.go` `EventStore.Subscribe()` | No callers anywhere (only `SubscribeAndList` is used, by `WatchEvents`). | 4 |
| `adapters/tui/tabs/net.go` `webURL` | Alias of `publicBaseURL`; one caller. Inline. | 1 |
| `adapters/tui/tabs/logs.go` `LogsTab.keys` | Assigned in `NewLogsTab`, never read (`Update` receives keys as a parameter). Drop the field. | 2 |
| `core/runtime/types.go` `Stopping`, `Failed`, `Unknown` states | Never assigned or compared anywhere. | 3 |
| `adapters/tui/run.go` | `finalModel, err := ...Run(); _ = finalModel` → `_, err :=`. | 2 |
| `adapters/cli/commands.go:101` | `entry.MaxArgs >= 0 &&` — no registry entry has negative MaxArgs; the `-1 = unlimited` convention only exists in `cmd/cli.go argsValidator`. Either drop the guard or drop the convention. | 1 |
| `core/setup/defaults.go` (whole file) | Four package vars that just alias `config.*` constants. Inline at the ~6 use sites. | 13 |
| `core/modelcatalog/types.go` `File.EstimatedVRAMBytes` | Never assigned — but the web UI reads it; **populate, don't delete** (see checkpoint 5). | — |
| `core/modelcatalog/types.go` `MachineProfile.GPUName/VRAMBytes/HasGPU` | Never set — but the web UI reads them; **populate from the signals collector, don't delete** (see checkpoint 5). | — |
| `core/modelcatalog/types.go` `ListResult.Errors` | Never appended to (see Improvements: per-model fetch errors are silently swallowed). Populate or delete. | ~3 |

### Proto (fields never set or never read by the Go server)

All in `core/rpc/proto/v1/`:

- `models.proto` `ModelRef` — message has zero references in hand-written Go; `ResolveModelResponse.model = 2` is never populated. Delete message + field (reserve tags).
- `models.proto` `StartModelDownloadRequest.model = 1`, `target_profile = 2` — never read (`StartModelDownload` uses url/filename/force only).
- `models.proto` `ListModelCatalogRequest.query = 1` — kept only as a deprecated fallback for `search`; mark deprecated or remove.
- `signals.proto` `ListEventsRequest.limit = 1`, `after_id = 2` — `ListEvents` ignores its request entirely.
- `common.proto` `GetInfoResponse.version = 3`, `llamarig_home = 4`, `values = 10` — never set by `GetInfo`.
- `common.proto` `RouterInfo` starts at field 2 with no `reserved 1` — cosmetic, but reserve it.

Removing these shrinks the generated `.pb.go` code too (the biggest LOC pool in the repo).

## Checkpoint 2 — LOC reduction opportunities (no behavior change)

Ordered roughly by payoff/effort.

1. **Duplicate error-kind mapping tables** — the kind↔code↔status mapping is written
   three times as switches: `rpc.rpcError` (kind→connect code), `rpc.ErrorKindFromRPC`
   (code→kind), `public_http.httpStatusForKind` (kind→HTTP status). One
   `map[control.ErrorKind]struct{connect.Code; int}` plus a reverse lookup collapses
   ~70 lines to ~25 and guarantees the three stay in sync.

2. **Two "surgical config.yaml edit" implementations** — `config.SetStartupServices`
   does a regex block-replace while `configstore.mutateDocument` does proper
   yaml.Node surgery (comments preserved, validated, atomic backup). Move
   startup-services editing into configstore and delete the regex path
   (`startupServicesBlock`, `SetStartupServices` ≈ 25 lines, plus the weaker
   guarantees). The TUI already has an RPC client; it could even go through
   `ReplaceConfig`-style RPC instead of writing the file directly.

3. **Duplicate `commandRunner`/`execCommandRunner`** — identical interface+impl in
   `core/control/llama_help.go` and `core/signals/gpu_subprocess.go` (only
   CombinedOutput vs Output differs). One shared type in `platform/process` drops
   ~12 lines and one concept.

4. **Duplicate autostart-cap sentinel** — `config.ErrAutostartCapExceeded` and
   `configstore.ErrAutostartCapExceeded` both exist; configstore translates one into
   the other, then `control.mapConfigStoreError` maps again. Keep the `config` one,
   delete the configstore alias and its translation branch (~8 lines, one less hop).

5. **`public_http` auth duplication** — `requireBearerToken` (http.Handler) and
   `Server.requireAuth` (http.HandlerFunc) are the same check twice. Keep one,
   adapt the other call site (~12 lines).

6. **`logs_routes` / `cmd/command.go` hand-built archive maps** — `Archive` already
   has JSON tags; `logArchives` rebuilds `map[string]any` per item and
   `listLogArchives` hand-formats. Marshal the struct (rename `Service`→`source`
   handling once) and drop the loops (~15 lines).

7. **TUI poll errgroup** — `dashboardBackend.poll` uses `errgroup` but every
   closure returns nil; a `sync.WaitGroup` (or four sequential calls — the four
   RPCs share a 2s budget anyway and hit a local Unix socket) reads simpler and
   drops the `fetchResult`/`fetched` plumbing (~20 lines if sequential).

8. **`adapters/cli` action dispatch** — `runAction`/`callAction` re-switch on
   `c.name` although the registry already dispatches per command. Put the RPC call
   in the `CommandSpec` (like the other handlers) and delete `callAction` (~15 lines).

9. **`public_http.serveHTTP`** — `appHandler(s.appFS)` builds a new
   `http.FileServer` on every request. Build once in `NewServer` (also a micro-perf
   win), and the path-exclusion condition can move into mux registration.

10. **Small stuff** — `tui/model.go Update` tail (`if cmd := ...; cmd != nil {...}
    return m, nil` → `return m, m.tabs.Update(msg)`); `writeCommandRPCResponse` has
    one caller (inline); `cmd/command.go` commented-out `Short:` on `tuiCommand`;
    `setup/model.go` `_ = ctx` (drop the unused param instead).

## Checkpoint 3 — Bugs and behavioral risks

1. **`platform/process/detached.go` `StartDetached` — nil-func panic.**
   ```go
   closeLogs, err := audit.AttachLogs(cmd, name)
   defer closeLogs()
   if err != nil { return err }
   ```
   `AttachLogs` returns `(nil, err)` on failure, so the deferred call panics with a
   nil dereference instead of returning the error. Move `defer closeLogs()` below
   the error check. (Trigger: any log-path failure, e.g. unwritable `~/.llamarig/run`.)

2. **`control/llama_help.go` holds the cache mutex across the subprocess.**
   `GetLlamaServerParams` locks `helpCache.mu`, then shells out with a 5s timeout.
   Concurrent callers (web UI param editor + MCP) serialize behind a potentially
   slow/hung `llama-server --help`. Snapshot-under-lock, run unlocked, re-lock to
   store — or accept it and document; either way it's a latency cliff on cache miss.

3. **`Manager.routerConfigSnapshot` re-reads and re-parses config.yaml on every call.**
   It's called from `GetInfo`, `Status` paths, preset listing (autostart sets), etc.;
   the TUI and web UI poll every ~5s, and `ListPresets` calls it once per request.
   Works, but it's disk I/O + YAML parse per poll. A modtime-checked cache in
   `configstore.FileStore` would centralize it.

4. **Per-model catalog fetch errors are swallowed.** `HuggingFaceCatalog.catalogModel`
   returns `ok=false, err=nil` when `fetchModelInfoRaw` fails, so a transient HF
   hiccup silently shrinks the catalog and gets cached for the TTL.
   `ListResult.Errors` exists precisely for this — populate it (also fixes the dead
   field in checkpoint 1). Similarly, `List` hard-fails when the cache file is
   corrupt (`read catalog cache: ...`) instead of falling back to a network fetch.

5. **`presetSourceCache` never evicts.** Entries keyed by preset name live forever
   (deleted presets included). Harmless at this scale, but a `delete` hook from
   preset deletion or a sweep would keep it honest.

6. **`modelcatalog.watchLocal` fingerprints via `fmt.Sprint(models)`.** Correct
   today because `LocalModel` is flat, but any added field with a pointer would
   quietly break change detection. A comment or an explicit key
   (`path|size|modtime` join) would be safer.

7. **`EventStore` drops events for slow subscribers silently** (non-blocking send,
   buffer 100). Fine as a design choice, but `WatchEvents` clients can miss events
   with no gap signal even though the protocol has `after_id` semantics for backlog.

## Checkpoint 4 — Structural improvements (larger, optional)

1. **Version the MCP server string.** `mcp.NewServer` hardcodes `Version: "v0.1.0"`
   and `GetInfoResponse.version` is never populated — there is no build version
   anywhere. A single `version` package (ldflags-injected) feeding `GetInfo`, MCP,
   and a `--version` flag would close that gap and un-dead the proto field.

2. **Unify the three control-socket client constructors.**
   `adapters/cli/client.go` (30s), `adapters/tui/tabs/client.go` (5s),
   `public_http/internal_client.go` (30s + streaming 0) are all one-line wrappers
   over `rpc.DialControl` with different timeouts. Not much LOC, but the timeout
   policy is scattered; consider constants next to `DialControl`.

3. **`public_http` is a hand-rolled REST façade over the RPC it already serves.**
   Half of `routes.go` adapts Connect RPCs to JSON endpoints one by one. Connect
   handlers already speak JSON over HTTP POST; long-term, exposing the Connect
   handler on the public server (behind the same auth/origin middleware) could
   remove most of `model_routes.go`/`preset_routes.go`/`request_response.go`
   (~500 LOC) at the cost of a breaking API change for the web UI. Worth a design
   discussion, not a quick win.

4. **TUI message plumbing.** Each action needs 4 pieces (request msg, result msg,
   backend cmd, selector-case). A generic `rpcRequestMsg{do func() error, done func(error) tea.Msg}`
   would collapse `modelDelete*/presetCleanup*/presetAutostart*/presetStart*`
   pairs (~60 LOC) — but Bubble Tea idiom favors explicit messages; only do this
   if more actions keep accruing.

5. **`config.Parse` vs `configstore.Validate` double-parse on write paths.**
   `Replace` validates (parse #1) then `replaceLocked` writes; the next `Read`
   parses again. Cheap, but if #3 in checkpoint 3 gets a cache, route all parsing
   through it.

## Summary

- Two real bugs: `StartDetached` nil-defer panic (fix first), help-cache lock
  convexity under contention.
- ~150–200 LOC of straightforward deletions/dedups in hand-written Go, plus a
  meaningful cut to generated proto code from removing dead messages/fields.
- Biggest architectural lever (optional): serving Connect directly on the public
  gateway instead of the hand-rolled REST adapter layer.

## Checkpoint 5 — Frontend cross-check of the API findings

The web UI (`webui/web/src`, hand-written parts) was read in full and used to
re-validate the checkpoint 1/2 API claims against the API's actual consumers
(web UI, TUI, CLI, MCP).

**Confirmed dead by the FE as well** (no consumer anywhere):

- `ModelRef` message and `ResolveModelResponse.model` — FE reads `resolution` only.
- `StartModelDownloadRequest.model` / `target_profile` — FE sends `{url, filename}`.
- `ListEventsRequest.limit` / `after_id` — FE calls `/api/events` with no params.
- `GetInfoResponse.version` / `llamarig_home` / `values` — FE `InfoResponse` type
  declares only `router` and `default_preset`.
- `ListModelCatalogRequest.query` — FE always sends `search`.
- `ListResult.Errors` — not even present in the FE `ModelCatalogResponse` type.
- **New:** `ApplyModelDownloadToPresetRequest.restart` — proto already marks it
  deprecated; FE never sends it. The `restart` branch in
  `ApplyModelDownloadToPreset` (model_rpc.go) and the `Restart` field in
  `public_http.modelApplyRequest` can go with it.

**Flipped — do NOT delete, populate instead:**

- `MachineProfile.vram_bytes` / `has_gpu` / `total_ram_bytes` and
  `ModelFile.estimated_vram_bytes`: the FE actively consumes these
  (`estimateLocalFit`, `rankedResourceSummary`, `capacityLabel`, and the catalog
  footprint meter read `best_file.estimated_vram_bytes || estimated_ram_bytes`).
  Because the backend never fills them, the FE silently falls back to `/api/signals`
  GPU data on every render. Wiring the signals collector's GPU stats into
  `GopsutilMachineProfiler` (or the fit estimator) makes the existing FE code work
  as designed. Checkpoint 1 is amended accordingly.

**New API-surface findings from the FE read:**

- `GET /api/runtime/resources` has no consumer: the FE uses `/api/signals`, the
  TUI calls the `GetRuntimeResources` RPC directly over the socket. The REST route
  (and `llamaParamsPayload`-style plumbing around it) can be dropped; keep the RPC.
- `GET/PUT /api/config.yaml` + `POST /api/config.yaml/validate` have no FE
  consumer at all — and the FE's `CodeEditor.svelte` + `editorTheme.ts` (a YAML
  CodeMirror editor, clearly built for this) are themselves dead, imported only by
  their own test. Decide the feature's fate once: either ship the config editor
  panel, or delete the three routes, `config_routes.go`, `CodeEditor.svelte`,
  `editorTheme.ts`, and their tests (~350 LOC across Go+TS).
- `DELETE /api/models/local` cascade confirmation: both FE and TUI always send
  `cascade_presets=true`, so the "referenced by presets; confirm" conflict path in
  `Manager.DeleteLocalModel` is reachable only via raw curl. Fine to keep, but it
  is untested by any real client.

**FE-side bugs noticed during the cross-check** (fix in webui, not the API):

- `RuntimePanel` treats an operation as OK when `status ∈ {ok, success, done}`,
  but the API emits `succeeded` / `failed` / `skipped` — successful operations
  render with destructive styling. `ModelsPanel` likewise colors the download
  badge for `complete`/`done`, but the API states are `completed` /
  `already_downloaded`.
- Dead FE code: `percent()` inside `createLlamaRigClient` (unused), the
  `llamaRigState` singleton in `state.svelte.ts` (components use only the type +
  factory), and `panels/presets/presetTemplates.ts` (a pure re-export shim).

## Checkpoint 6 — How the ~500 LOC public_http cleanup works

Goal: stop hand-adapting every Connect RPC into a bespoke JSON route and let the
gateway pass Connect through. Connect's own protocol is JSON-over-HTTP-POST, so
the browser can speak it natively with connect-web.

Plan (single coordinated change; the web UI is embedded in the binary, so FE and
gateway always ship together — no compatibility window needed):

1. **Gateway: reverse-proxy Connect to the control socket (~25 new LOC).**
   The gateway is a separate process from the control daemon, so it can't mount
   the service handler in-process; instead add an `httputil.ReverseProxy` whose
   transport dials the Unix socket (same dialer as `rpc.DialControl`) and mount it
   at `/rpc/` (i.e. `/rpc/llamarig.control.v1.ControlService/*`). Wrap it in the
   existing `originGuard` + `requireBearerToken` middleware. Error mapping comes
   for free: Connect error codes and the `-Error-Kind` metadata header pass
   through untouched, so `writeRPCError`, `httpStatusForKind`, and the per-route
   nil-response boilerplate all become unnecessary for proxied routes.

2. **FE: generate a client from the protos.** Add `buf` + `protoc-gen-es` /
   `connect-web` codegen over `core/rpc/proto/v1`, and replace the hand-written
   methods in `api.ts` with a `createClient(ControlService, transport)` where the
   transport injects the bearer header. `WatchModelCatalog` becomes a Connect
   server-stream (async iterator) replacing the SSE endpoint + `EventSource`
   plumbing. Hand-written `types.ts` shrinks to a few view-model types; the
   generated types replace the rest.

3. **What stays REST.** Keep as-is, since they are implemented *in the gateway
   process*, not behind the RPC: `/api/logs*` (reads log files via
   `platform/audit` directly), `/health` fallback, static app serving, `/mcp`.

4. **Deletions once 1–2 land** (measured against current files):
   - `model_routes.go` (~130) — every route is a 1:1 RPC adaptation, including
     the hand-rolled SSE loop.
   - `preset_routes.go` (~72) and `config_routes.go` (~18).
   - Most of `request_response.go` (~110 of 140): body-limit readers, RPC
     response writers, error mappers. Keep `writeJSON`/`writeCoreError` for the
     log routes.
   - Adapter halves of `routes.go` and `runtime_routes.go` (~80): `rpcGet`,
     `runtimeAction`, `identity`, `llamaParamsPayload`, and the per-route lines.
   - Net: roughly 400–450 LOC of Go deleted against ~25 added, plus ~150 LOC of
     hand-written TS (`api.ts` request methods, SSE handling, duplicated types)
     replaced by generated code. That is where the "~500 LOC" figure comes from;
     it holds only if the config.yaml routes are also resolved (checkpoint 5).

5. **Risks / decisions.**
   - Any *external* consumer of the current REST API breaks. Known consumers are
     only the embedded FE, TUI (socket RPC), CLI (socket RPC), MCP (own protocol);
     if the REST API was ever documented for third parties, this needs a
     deprecation note instead of silent removal.
   - The gateway currently degrades gracefully when the control daemon is down
     (`internalControl == nil` checks). The proxy equivalent is a 502 from the
     failed dial — same UX, less code, but confirm the FE surfaces it sanely.
   - The validation interceptor keeps running in the daemon, so proxied requests
     lose nothing.
