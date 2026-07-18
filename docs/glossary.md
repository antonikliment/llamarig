# Glossary

- **Active log** — Current writable log for a running or most recently started LlamaRig service.
- **Log archive** — Read-only timestamped log moved from the active path during detached startup.
- **Rotation** — Moving a non-empty active log into the archive before opening a new active file.
- **Retention** — Maximum archive age before automatic cleanup. `0s` means unlimited retention.
- **Tail** — Bounded last-N-line view of a current or archived log.
- **App-owned REST** — The web gateway's `/api/*` facade for its bundled web UI. It is versioned with the app, not maintained as a third-party compatibility API.
- **ControlService** — Internal RPC contract served over the local control socket and used by LlamaRig protocol adapters.
