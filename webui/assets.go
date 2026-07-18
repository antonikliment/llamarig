package webui

import "embed"

// Files contains the browser UI served by LlamaRig.
//
//go:embed dist/index.html dist/assets/*
var Files embed.FS
