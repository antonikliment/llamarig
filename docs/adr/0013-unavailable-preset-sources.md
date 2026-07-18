# Unavailable Preset sources

## Status

Accepted.

## Context

The Router accepts Presets whose local `model` or `models-dir` path later
disappears. Loading one starts a llama.cpp child that exits during model load.
Automatically commenting the source document would destroy user intent, while
a filtered runtime copy would add a second `models.ini` lifecycle.

## Decision

- Root `models.ini` remains the sole durable Preset source.
- `core/modelpresets` inspects local source availability and owns reference
  classification for exact model paths and model directories.
- Control blocks Start and Restart for unavailable Presets before Router load.
- Protocol adapters expose source status and keep unavailable Presets visible.
- Explicit cleanup removes an unavailable Preset and matching
  `default_preset`/`autostart_presets` references.
- Deleting a local model may cascade only through exact `model` references
  after confirmation. `models-dir` Presets remain. Loaded Presets block the
  cascade.

## Consequences

- Missing local files no longer spawn doomed model children through LlamaRig.
- External deletion remains safe and visible without rewriting `models.ini`.
- Cleanup is destructive but explicit, confirmed, and retryable.
- A file can still disappear between preflight and llama.cpp opening it; normal
  Router error handling covers that unavoidable race.
