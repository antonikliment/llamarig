package e2e

import (
	"context"
	"testing"

	controlv1 "llamarig/core/rpc/gen/v1"
)

func TestCatalogListAndResolve(t *testing.T) {
	client := startService(t)
	ctx := context.Background()

	catalog, err := client.ListModelCatalog(ctx, &controlv1.ListModelCatalogRequest{Search: "tiny", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "ListModelCatalog", catalog.GetOk())
	if catalog.GetMachine() == nil || catalog.GetCache() == nil {
		t.Fatalf("catalog missing machine/cache shape")
	}

	resolved, err := client.ResolveModel(ctx, &controlv1.ResolveModelRequest{Model: "https://huggingface.co/ggml-org/models"})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "ResolveModel", resolved.GetOk())
	if resolved.GetResolution() == nil || len(resolved.GetResolution().GetFiles()) == 0 {
		t.Fatalf("resolution missing files")
	}
}
