# ADR 0008: shadcn-svelte owns generic web UI

LlamaRig uses Tailwind CSS v4 and copied shadcn-svelte modules as the generic web UI layer. Feature panels compose registry modules directly, while semantic theme variables preserve LlamaRig branding and `mode-watcher` provides system-default light and dark modes. We avoid a parallel LlamaRig wrapper layer because it duplicates the registry interface, weakens accessibility defaults, and recreates the maintenance cost this migration removes; custom frontend modules remain appropriate only for domain behavior the registry does not provide.
