//go:build e2e_live

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	controlv1 "llamarig/core/rpc/gen/v1"
)

func TestLiveDownloadSpawn(t *testing.T) {
	llamaServer := resolveLlamaServer(t)
	routerPort := freePort(t)
	t.Setenv("E2E_LLAMA_SERVER", llamaServer)
	t.Setenv("E2E_ROUTER_PORT", fmt.Sprint(routerPort))
	client := startService(t)
	ctx := context.Background()
	name := "e2e-live"
	writePreset(t, name, map[string]string{"ctx-size": "512"})

	started, err := client.StartModelDownload(ctx, &controlv1.StartModelDownloadRequest{
		Url:      "https://huggingface.co/ggml-org/models",
		Filename: "tinyllamas/stories260K.gguf",
		Force:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "StartModelDownload", started.GetOk())
	download := waitDownload(t, client.GetModelDownload, started.GetDownload().GetId())
	if download.GetState() != "completed" {
		t.Fatalf("download state=%q error=%q", download.GetState(), download.GetError())
	}

	applied, err := client.ApplyModelDownloadToPreset(ctx, &controlv1.ApplyModelDownloadToPresetRequest{
		Id:     download.GetId(),
		Preset: name,
	})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "ApplyModelDownloadToPreset", applied.GetOk())
	startedRuntime, err := client.StartRuntime(ctx, &controlv1.RuntimeTargetRequest{Target: name})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "StartRuntime", startedRuntime.GetOk())
	t.Cleanup(func() {
		// Empty target stops the supervised router process itself, not just the
		// loaded model, so the spawned llama-server does not outlive the test and
		// hold its stdio pipes open.
		_, _ = client.StopRuntime(context.Background(), &controlv1.RuntimeTargetRequest{Target: ""})
	})

	waitRuntimePreset(t, client.GetRuntimeStatus, name)
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", routerPort))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/health status = %d", resp.StatusCode)
	}

	stopped, err := client.StopRuntime(ctx, &controlv1.RuntimeTargetRequest{Target: name})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "StopRuntime", stopped.GetOk())
}

func waitRuntimePreset(t *testing.T, get func(context.Context, *controlv1.GetRuntimeStatusRequest) (*controlv1.GetRuntimeStatusResponse, error), name string) *controlv1.RuntimePreset {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
			status, err := get(ctx, &controlv1.GetRuntimeStatusRequest{})
			if err != nil {
				t.Fatal(err)
			}
			preset := findPreset(status.GetStatus().GetPresets(), name)
			if preset != nil && preset.GetState() == "running" && preset.GetReady() {
				return preset
			}
		}
	}
}
