# Web RPC migration

## Goal

Remove the browser-specific REST facade for control operations and use the
existing Connect RPC contract end to end. Keep the public gateway responsible
for origin checks, bearer authentication, static assets, MCP, health/info, and
log routes.

## Plan

1. Proxy the existing control service through the public gateway over its Unix
   socket, preserving the existing public-read/private-write authorization
   policy.
2. Delete the duplicated Go REST handlers and migrate gateway tests to Connect.
3. Proceed only if Go implementation code falls by at least 100 gocloc lines.
4. Generate a TypeScript client from the existing protobuf schema and migrate
   the Svelte API facade and catalog stream.
5. Run Go and web verification, update this status, then publish a draft PR.

## Status

- [x] Isolated workspace and branch created (`agent/rpc-web-migration`).
- [x] Public Connect proxy implemented with origin and method-level auth.
- [x] Handwritten model, preset, and runtime REST adapters removed.
- [x] Go gate passed: 9,730 to 9,520 implementation LOC (-210); public HTTP
  complexity fell from 137 to 96.
- [x] Go milestone committed (`3360c0e`) and pushed.
- [x] Svelte API facade migrated to Connect-ES; catalog refresh uses the typed
  server stream instead of `EventSource`.
- [x] Generated TypeScript stays untracked and is reproduced by the web scripts
  with `protoc-gen-es`; `pnpm run verify:web` passes.
- [ ] Draft PR opened.

The `/health`, `/info`, `/api/logs`, and `/mcp` HTTP surfaces remain because
they are gateway-owned concerns or intentionally outside the control schema.
The selected libraries are the official `@bufbuild/protobuf`,
`@bufbuild/protoc-gen-es`, `@connectrpc/connect`, and
`@connectrpc/connect-web` packages; no Svelte-specific RPC wrapper is needed.
