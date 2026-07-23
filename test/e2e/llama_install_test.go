//go:build e2e_live

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"llamarig/config"
	"llamarig/core/llamainstall"
)

func resolveLlamaServer(t *testing.T) string {
	t.Helper()
	if path := os.Getenv("LLAMA_SERVER"); path != "" {
		requireExecutable(t, path)
		return path
	}
	if path, err := exec.LookPath("llama-server"); err == nil {
		return path
	}
	return installLatestLlamaServer(t)
}

func installLatestLlamaServer(t *testing.T) string {
	t.Helper()
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	t.Setenv(config.ProjectHomeEnv, filepath.Join(cache, "llamarig", "e2e"))
	backend := llamainstall.BackendCPU
	if runtime.GOOS == "darwin" {
		backend = llamainstall.BackendMetal
	}
	path, err := llamainstall.Install(context.Background(), llamainstall.Options{
		Backend: backend, BackendSet: true, Progress: os.Stderr,
	})
	if err != nil {
		t.Fatalf("install managed llama.cpp: %v", err)
	}
	requireExecutable(t, path)
	return path
}

func requireExecutable(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("llama-server %q is not executable: %v", path, err)
	}
}
