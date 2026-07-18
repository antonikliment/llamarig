# Dashboard UX

LlamaRig's Dashboard is the existing runtime control surface reorganized for quick operation. It keeps LlamaRig's shadcn-svelte components and semantic theme tokens while borrowing the reference dashboard's information order: health, active workloads, current resources, then trends and details.

## V1 decisions

- `runtime` remains the internal section identifier; its user-facing name is Dashboard.
- Summary cards show runtime health, active preset capacity, local model count, and telemetry freshness.
- Active presets expose confirmed per-preset stop and restart actions. Starting inactive presets remains owned by Presets.
- CPU, memory, GPU utilization, VRAM, and optional GPU temperature retain five minutes of browser-local samples at the existing five-second polling interval.
- LayerChart renders accessible live trends with exact-value tooltips. Reloading the page clears history.
- Failed refreshes retain the last successful snapshot and mark it stale.
- Layout stacks without hiding metrics on narrow screens.

## Future analytics

Persistent telemetry is intentionally deferred. Before adding it, decide retention, storage ownership, sampling/downsampling, and time-range contracts. Request volume, latency, error distribution, and token throughput also require a request-observation source that LlamaRig does not currently expose. Those additions should extend the dashboard rather than change its status-and-control hierarchy.
