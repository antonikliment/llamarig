# Design note: webapp asset reload & binary self-update

Status: **open** — captures the option space for a future decision. Nothing here is committed.

## Problem

LlamaRig ships as a single binary. The web UI bundle is `//go:embed`-ed into that
binary (`webui/assets.go` → `webui.Files`, served via `bootstrap/service.go`
`appFileSystem`). Two related questions came up:

1. During **development**, can a fresh `pnpm build` be reflected without restarting the Go server?
2. During a **production upgrade**, how does the new JS reach a running server?

## Key constraint: embedded assets are frozen in a live process

Once the process is running, the embedded bytes are part of its memory image.
On Linux/macOS, replacing the binary file on disk does **not** affect the running
process (it keeps the old inode). On Windows the running `.exe` is locked outright.
So **embedded JS cannot change in-flight** — it only changes when the process is
replaced/restarted. There is no hook to hot-swap embedded assets.

## Two serving modes already exist

`bootstrap/service.go` `appFileSystem()`:

- **Embedded (default):** `fs.Sub(webui.Files, "dist")` — baked into the binary.
- **Dir mode:** if `LLAMARIG_APP_DIR` is set, `os.DirFS(dir)`. `http.FileServer`
  opens files fresh per request, so a rebuild on disk is served on the next
  request with no server restart (browser still needs a manual refresh).

### Dev reload (decided direction, dev-only)

Live reload is inherently a **dir-mode / dev-only** feature — in production there
is no loose JS on disk to watch. The chosen approach (not yet implemented at time
of writing) is an SSE `/__livereload` endpoint that watches `webui/dist`
(fsnotify) and a tiny `<script>` injected into `index.html` **only in dir/dev
mode**, absent from the embedded build. Alternatives considered: Vite dev server
HMR (`pnpm --dir webui dev`, proxies `/api` etc. to `:7000`) — best inner loop,
no Go changes; and plain dir mode with manual refresh — zero code.

## Production: how new JS reaches a running server

Because the JS is embedded, **the binary swap *is* the JS update** — frontend and
backend flip together atomically, which avoids version skew. Three ways to apply
an upgrade to a running server:

1. **Restart the process** (normal answer). New binary → new embedded JS on next
   start. Owned by a supervisor (systemd/launchd/Windows Service/container
   orchestrator) or a self-update that re-execs.
2. **Move JS out of the binary** (dir mode in production). `os.DirFS` serves fresh,
   so swapping files needs no restart — but you ship binary + asset dir as two
   artifacts and risk **frontend/backend version skew** (new JS vs old `/api`).
3. **Graceful handoff** — new process takes over the listener while the old drains.
   Functionally still a restart.

## Self-update: Go code vs. system supervisor

"Self-update" is two separable acts:

### 1. Swap the binary on disk — Go can do this cross-platform
- **Linux/macOS:** write new binary to a temp file on the same filesystem, atomic
  `os.Rename` over the target. Old running process unaffected until restart.
- **Windows:** can't overwrite/delete a running `.exe`, but *can* rename it: rename
  running exe to a sidecar, write new exe to the original name, delete sidecar on
  next launch.
- Library: `minio/selfupdate` (maintained fork of `inconshreveable/go-update`)
  implements this dance incl. the Windows rename and checksum/signature
  verification. Verify current API before adopting.

### 2. Restart to apply — platform-specific
- **Linux/macOS:** `syscall.Exec` (execve) replaces the process image in place —
  same PID, re-runs `main()`. For zero-downtime, pass the listener FD across exec
  (`cloudflare/tableflip`, `jpillora/overseer`); otherwise connections blip.
- **Windows:** no execve — must `os.StartProcess` a new process and exit the old
  (start-new-then-die). Socket handoff is harder; usually defer to the supervisor.

### Which lives where
- **In-process Go self-update** fits **per-user installs** the user owns the binary
  for (cf. `gh`, syncthing, caddy, k3s): `minio/selfupdate` to swap + re-exec on
  POSIX / spawn-and-exit on Windows. Self-contained.
- **Supervisor-owned upgrade** fits a **system service**, because in-process
  self-replace hits three packaging landmines:
  - **Permissions:** admin-owned paths (`/usr/local/bin`, `C:\Program Files`) can't
    be overwritten by an unprivileged service.
  - **Code signing:** self-overwrite breaks macOS notarization/Gatekeeper and
    Windows Authenticode unless re-signed; Homebrew/MSI/Sparkle expect to own the file.
  - **Supervisors already restart correctly** (systemd/launchd/SCM/Docker); in a
    container you roll a new image and never self-update.

### Recommended hybrid (if/when we build self-update)
Let Go do the **portable** part and delegate the **platform** part:

> Go downloads → verifies checksum/signature → stages the new binary → then
> **requests a restart** from its supervisor (`syscall.Exec` on POSIX if standalone,
> or `systemctl restart` / launchd / `sc` / let the orchestrator recreate the container).

This decouples "fetch" (one portable Go path) from "apply" (platform-appropriate)
and sidesteps the running-exe lock and signing issues.

## Open decision

Pick the target install model — **per-user binary** vs. **managed system service**
(vs. **container image**) — since that determines whether self-update lives in Go
code or in the supervisor. Until then, only the dev-only dir-mode live reload is in
scope.
