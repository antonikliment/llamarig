package e2e

import (
	"context"
	"strings"
	"testing"

	controlv1 "llamarig/core/rpc/gen/v1"
)

func TestConfigReplaceRoundTrip(t *testing.T) {
	client := startService(t)
	ctx := context.Background()

	current, err := client.GetConfig(ctx, &controlv1.GetConfigRequest{})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "GetConfig", current.GetOk())

	next := strings.Replace(current.GetContent(), "catalog_cache_ttl: 1m", "catalog_cache_ttl: 2m", 1)
	valid, err := client.ValidateConfig(ctx, &controlv1.ValidateTextDocumentRequest{Content: next})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "ValidateConfig", valid.GetOk())

	replaced, err := client.ReplaceConfig(ctx, &controlv1.ReplaceTextDocumentRequest{Content: next})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "ReplaceConfig", replaced.GetOk())

	roundTrip, err := client.GetConfig(ctx, &controlv1.GetConfigRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(roundTrip.GetContent(), "catalog_cache_ttl: 2m") {
		t.Fatalf("updated config missing new ttl: %q", roundTrip.GetContent())
	}
}
