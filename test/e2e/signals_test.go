package e2e

import (
	"context"
	"testing"

	controlv1 "llamarig/core/rpc/gen/v1"
)

func TestSignalsSnapshot(t *testing.T) {
	client := startService(t)

	signals, err := client.GetSignals(context.Background(), &controlv1.GetSignalsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "GetSignals", signals.GetOk())
	snapshot := signals.GetSignals()
	if snapshot.GetCapturedAt() == "" || snapshot.GetCpu() == nil || snapshot.GetMemory() == nil {
		t.Fatalf("signals snapshot incomplete: %#v", snapshot)
	}
	if len(snapshot.GetDisks()) == 0 {
		t.Fatalf("signals disks empty")
	}
}
