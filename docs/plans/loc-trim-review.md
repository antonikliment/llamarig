# LOC Trim Review

Deep review of the Go production code for line-count reduction without losing
features. Scope: Go only (JS/Svelte excluded).

## Implementation result

Lever #1 is implemented. Production Go now measures **9,881 code lines**, down
from the post-review baseline of 9,986 (-105) and the original 9,999 (-118).
The work landed in five verified milestones:

1. `096b3c5` removes signals field-by-field reshaping.
2. `c366074` serializes model resolution and catalog children directly.
3. `9c43870` aligns model download RPC fields with the REST contract.
4. `ba8fa2d` aligns audit and catalog event RPC fields with REST.
5. `200b47a` removes the final model catalog compatibility map.

Each milestone passed `make test`, `make lint`, and `make e2e-live` before being
pushed. Generated protobuf output remains uncommitted.

One review assumption needed refinement: `protojson` emits 64-bit integers as
JSON strings. REST responses containing byte counts therefore use standard Go
JSON serialization over generated structs, preserving the existing numeric
contract without restoring field-by-field maps. Proto-only event responses use
`protojson` directly.

## Headline finding

The codebase is **already at its LOC floor and exceptionally well-factored**.
Measured baseline: **9,999 / 10,000** gocloc code lines (the budget enforced by
`goclocbudget`). At review time: `deadcode` finds nothing, `staticcheck U1000`
finds nothing, the custom linter reports `0 issues`, and `dupl` at threshold 50
surfaces only 3–9 line clones — the kind `AGENTS.md` explicitly says to leave
(“three similar lines is better than premature abstraction”).

So there is **no pile of easy, safe LOC to delete**. Real reduction requires
structural change (below). This review applied the safe micro-trims and leaves
the structural calls documented for a decision.

## Applied now (safe, tests + lint green)

Net: **9,999 → 9,986** code lines (−13), all tests pass, `0` lint issues.

1. **`GetSignals` / `GetRuntimeResources` share a `snapshot` helper**
   (`core/rpc/runtime_rpc.go`, `misc_rpc.go`). Both hand-rolled the identical
   nil-check + capture + error-map block with the *same* message. Zero behavior
   change.
2. **`health()` / `configGet()` use `writeRPCMappedResponse`**
   (`adapters/public_http/runtime_routes.go`, `config_routes.go`). They were the
   only two handlers hand-rolling the map/nil/error pattern every sibling
   handler already delegates. Success path and the nil-response→502 path are
   unchanged (covered by `TestHTTPNilRPCResponsesReturnRuntimeErrors` and
   `TestConfigGetUsesInternalRPCSocket`).
   - **Minor behavior refinement:** on a *non-nil* internal RPC error these two
     endpoints now return the mapped error kind/message instead of a blanket
     `502 "failed to call internal … rpc"`. Not test-covered; strictly more
     consistent with every other endpoint. Called out in case the web client
     keys off the old generic shape.

## Big levers

These are where the LOC actually is. Each is a real reduction but changes a
contract or an architectural boundary, so they are **not** “simple bits.”

### 1. Collapse the proto → `map[string]any` REST reshaping layer (implemented)

`adapters/public_http/{model_rest,runtime_rest}.go` plus inline mappers in
`model_routes.go` / `logs_routes.go` hand-convert every proto field into a
`map[string]any` with a snake_case key (~61 map literals). But `writeJSON`
already marshals proto via `protojson.MarshalOptions{UseProtoNames: true}`,
which **produces snake_case automatically** — that is exactly why `health`/
`configGet`/`info`/`runtimeStatus` can return the proto directly.

The reshaping survives only because a few fields are *renamed/reshaped* vs the
proto:
- `ModelDownload.downloaded_bytes` → REST `received_bytes`
- `ModelCatalogEvent.message` → REST `ok` + `error` split
- `Event.type` + flattened `fields` map → REST `action`/`success`/`duration`

**Result:** RPC field names/shapes now match the established REST contract.
Wire field numbers were retained or reserved, nested generated structs are
serialized directly, and the Svelte contract remains unchanged.

### 2. Collapse `core/control` domain structs into proto DTOs (~120 LOC)

`core/rpc/*.go` contains ~15 `…Proto` functions doing field-by-field copies from
`control.*` / `signals.*` / `modelcatalog.*` structs into `controlv1.*`. Several
domain structs (e.g. `control.RuntimeInfo`, `RuntimeStatus`, `RuntimePreset`)
are near-identical to their proto twins and exist mainly to carry `json` tags
and be the manager’s return type.

**Move (options):** (a) have `core/control` return proto types directly at the
manager boundary and drop the hand mappers; or (b) generate the mappers. Saves
~100–120 LOC of pure copying.
**Cost/risk:** erodes ADR-0005/0010’s “domain types independent of transport”
boundary — a deliberate architectural stance. This is a genuine tradeoff, not a
cleanup. Decide explicitly before touching.

### 3. Per-endpoint `if s.<dep> == nil { … "not configured" }` guards (~20 LOC)

~8 copies across `core/rpc/*.go`. A `requireDep(cond, msg)` helper would shave
~2 lines each. Deliberately **not** applied: each is 3 self-documenting lines and
collapsing them trades clarity for ~16 lines — against the project’s stated
“three similar lines” guidance. Listed only for completeness; recommend leaving.

## What was checked and found already-lean (no action)

- `core/modelcatalog/huggingface.go` (594) — dense but every function earns its
  place (HF fetch, fit scoring, README summarize, path safety).
- `platform/audit/logs.go` (407) — correctness-critical (cross-device move,
  symlink guards, chunked tail). Trimming here risks bugs.
- `core/rpc` mapping — already uses generic `mapProto`/`restMap` helpers and
  extracted `runtimeOp` / `modelDownloadResponse`. No further safe factoring.

## Recommendation

Lever #1 and the safe micro-trims are complete. Treat lever #2 as an explicit
ADR-level decision, not a refactor. Leave the dependency guards unchanged.
