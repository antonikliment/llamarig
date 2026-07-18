package e2e

import (
	"context"
	"testing"
	"time"

	controlv1 "llamarig/core/rpc/gen/v1"
)

func TestEventsAndWatch(t *testing.T) {
	client := startService(t)
	name := "event-runtime"
	writePreset(t, name, stubPresetEntries(t))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	started, err := client.StartRuntime(ctx, &controlv1.RuntimeTargetRequest{Target: name})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "StartRuntime", started.GetOk())
	t.Cleanup(func() {
		_, _ = client.StopRuntime(context.Background(), &controlv1.RuntimeTargetRequest{Target: name})
	})

	stream, err := client.WatchEvents(ctx, &controlv1.WatchEventsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan *controlv1.Event, 4)
	go func() {
		for stream.Receive() {
			events <- stream.Msg()
		}
	}()
	select {
	case event := <-events:
		if event.GetId() == "" {
			t.Fatalf("watched event missing id")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	listed, err := client.ListEvents(context.Background(), &controlv1.ListEventsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, "ListEvents", listed.GetOk())
	if len(listed.GetEvents()) == 0 {
		t.Fatalf("event list empty")
	}
}
