# ADR 0003: Profile-owned models.ini

## Status

Superseded by removal of managed profile `models.ini`.

## Context

ADR 0001 treated `core/modelsini` as an independent core module. In practice, `models.ini` is only used as a subdocument of a server profile: profile reads, validation, replacement, deletion, and command preview all flow through `core/serverconfigs`.

Keeping a standalone package duplicated durable file-write mechanics and made callers learn a separate module that did not own an independent lifecycle.

## Decision

`models.ini` belongs to `core/serverconfigs`.

`core/serverconfigs` owns the `ModelsINI` document type, normalization, syntax validation, profile-scoped reads, profile-scoped writes, and profile-scoped deletes. HTTP and MCP continue to expose the same wire shape, but no longer import a separate `core/modelsini` package.

Shared durable file mechanics live in `platform/filedoc`.

## Consequences

- `core/modelsini` is removed.
- Profile file behavior has one owner.
- Future structured `models.ini` parsing should be added inside `core/serverconfigs` unless `models.ini` gains a real lifecycle outside server profiles.

## Supersession

LlamaRig no longer owns profile-scoped `models.ini` as a managed document. Profile configuration is now `base.yaml` only. Generic llama-server `models_preset` remains available through explicit `base.yaml` fields.
