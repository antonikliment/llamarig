# Redundancy Review Follow-Ups

Findings from a full architecture review (2026-07) that were left unfixed
because they're lower priority than the items already applied on
`refactor-clear` (proto message consolidation, catalog cache atomic writes,
shared log-tail constant). Recorded here so they aren't re-discovered from
scratch next time.

## 1. Two hand-rolled pub/sub brokers with identical fan-out logic

- `core/modelcatalog/events.go:18-49` (`refreshBroker`)
- `core/control/events.go:21-97` (`EventStore`)

Both implement the same shape: `sync.Mutex` + `map[chan T]struct{}` as the
subscriber set, a `Subscribe() (<-chan T, func())` whose cleanup closure
deletes from the map and closes the channel, and a non-blocking
`for ch := range subs { select { case ch <- v: default: } }` publish loop.
`EventStore` additionally keeps a capped ring buffer with replay
(`SubscribeAndList`), which is the only material difference.

**Suggested fix:** extract a small generic `Broker[T]` (subscribe/unsubscribe/
publish) that both packages embed; `EventStore` layers its ring buffer on
top. Not urgent — there are only two instances, and a generic abstraction
for two call sites is a judgment call. Worth doing if a third broker-shaped
thing shows up (e.g. for model download progress events).

## 2. Two byte-formatters in the same TUI package (FIXED)

Fixed on `refactor-clear`: `system.go`'s `gib` (1024 divisor, "GB" label)
was deleted in favor of `formatBytes` in `adapters/tui/tabs/models.go`.

There's a third, unrelated `gib` in `core/modelcatalog/fit.go:37` —
`func gib(value int64) float64` — used for interpolating byte counts into
human-readable reason strings. It can't share code with the TUI formatters
without introducing a cross-layer dependency (core can't import an
adapter's presentation helper, and the adapter shouldn't import
`core/modelcatalog` just for a formatter). Only worth centralizing if a
neutral `platform/units`-style package is ever justified by more callers.

## 3. Two stale-while-revalidate cache patterns, structurally similar but diverged

- `core/modelcatalog/huggingface.go:148-177` (`refreshAsync`): in-flight
  guard is a `map[string]struct{}` under a mutex; publishes a `RefreshEvent`
  via `refreshBroker` after the refresh completes.
- `core/rpc/preset_source_cache.go:30-60` (inline in `status()`/`refresh()`):
  in-flight guard is an `entry.refreshing bool` field under the same
  mutex as the cache map; writes the result back into the entry directly,
  no event publish.

Same idea (serve stale data immediately, kick off a background refresh,
prevent duplicate concurrent refreshes for the same key), reinvented twice
with different in-flight-tracking mechanics. Not consolidated because the
two are different enough (event-publishing vs. direct write-back) that a
shared helper would be more abstraction than the two call sites justify.

**Suggested fix:** leave as-is unless a third stale-while-revalidate cache
gets added — at that point extract a shared `staleCache[K, V]` helper
covering all three.
