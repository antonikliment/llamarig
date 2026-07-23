package tabs

import (
	"strings"
	"testing"

	controlv1 "llamarig/core/rpc/gen/v1"
)

func TestSystemResourcesDetailPanelRendersAllDisks(t *testing.T) {
	view := systemResourcesDetailPanel(80, 10, &controlv1.SignalsSnapshot{Disks: []*controlv1.DiskSnapshot{
		{Label: "root", Path: "/", UsedPercent: 41, UsedBytes: 41 * 1024 * 1024 * 1024, TotalBytes: 100 * 1024 * 1024 * 1024},
		{Label: "model_storage", Path: "/models", UsedPercent: 42, UsedBytes: 194 * 1024 * 1024 * 1024, TotalBytes: 458 * 1024 * 1024 * 1024},
	}}, "")

	for _, want := range []string{"Root:", "/", "Models:", "/models", "(41.0 GiB/100.0 GiB)", "(194.0 GiB/458.0 GiB)"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail panel missing %q:\n%s", want, view)
		}
	}
}

func TestSystemResourcesPanelRendersSocketWarning(t *testing.T) {
	view := systemResourcesDetailPanel(80, 8, nil, "dial unix control.sock: connect: no such file")

	if !strings.Contains(view, "Warning: control socket not available") {
		t.Fatalf("panel missing socket warning:\n%s", view)
	}
}

func TestSystemResourcesPanelRendersRPCWarningsWhenGPUUnavailable(t *testing.T) {
	view := systemResourcesDetailPanel(80, 8, &controlv1.SignalsSnapshot{Warnings: []string{"GPU telemetry unavailable"}}, "")

	if !strings.Contains(view, "GPU telemetry unavailable") {
		t.Fatalf("panel missing RPC warning:\n%s", view)
	}
}

func TestResourceMeterClampsPercent(t *testing.T) {
	for _, percent := range []int{-1, 101} {
		if got := resourceMeter.View(percent); got == "" {
			t.Fatalf("meter for %d is empty", percent)
		}
	}
}
