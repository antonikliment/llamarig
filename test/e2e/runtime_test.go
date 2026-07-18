package e2e

import (
	"context"
	"testing"

	controlv1 "llamarig/core/rpc/gen/v1"
)

func TestRuntimeLifecycleStub(t *testing.T) {
	client := startService(t)
	ctx := context.Background()
	name := "stub-runtime"
	writePreset(t, name, stubPresetEntries(t))

	started, err := client.StartRuntime(ctx, &controlv1.RuntimeTargetRequest{Target: name})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "StartRuntime", started.GetOk())
	t.Cleanup(func() {
		_, _ = client.StopRuntime(context.Background(), &controlv1.RuntimeTargetRequest{Target: name})
	})

	status, err := client.GetRuntimeStatus(ctx, &controlv1.GetRuntimeStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	preset := findPreset(status.GetStatus().GetPresets(), name)
	if preset == nil || preset.GetState() != "running" || !preset.GetReady() {
		t.Fatalf("preset not running/ready: %#v", preset)
	}

	resources, err := client.GetRuntimeResources(ctx, &controlv1.GetRuntimeResourcesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "GetRuntimeResources", resources.GetOk())
	if resources.GetResources().GetTotalRamBytes() == 0 {
		t.Fatalf("runtime resources missing memory")
	}

	restarted, err := client.RestartRuntime(ctx, &controlv1.RuntimeTargetRequest{Target: name})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "RestartRuntime", restarted.GetOk())

	stopped, err := client.StopRuntime(ctx, &controlv1.RuntimeTargetRequest{Target: name})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "StopRuntime", stopped.GetOk())
}

func findPreset(presets []*controlv1.RuntimePreset, name string) *controlv1.RuntimePreset {
	for _, preset := range presets {
		if preset.GetName() == name {
			return preset
		}
	}
	return nil
}
