package e2e

import (
	"context"
	"testing"

	"llamarig/config"
	controlv1 "llamarig/core/rpc/gen/v1"
)

func TestSetStartupServicesRoundTrip(t *testing.T) {
	client := startService(t)
	response, err := client.SetStartupServices(context.Background(), &controlv1.SetStartupServicesRequest{Services: []string{config.StartupServiceControl}})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "SetStartupServices", response.GetOk())
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.StartupServices) != 1 || cfg.StartupServices[0] != config.StartupServiceControl {
		t.Fatalf("startup services = %v", cfg.StartupServices)
	}
}
