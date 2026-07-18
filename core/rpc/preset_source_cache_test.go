package rpc

import (
	"testing"
	"time"

	"llamarig/core/modelpresets"
)

func TestPresetSourceCacheInspectsOffRequest(t *testing.T) {
	release := make(chan struct{})
	cache := newPresetSourceCache()
	cache.inspect = func(modelpresets.Section) modelpresets.SourceStatus {
		<-release
		return modelpresets.SourceStatus{State: modelpresets.SourceUnavailable, Error: "missing"}
	}
	section := modelpresets.Section{Name: "demo", Values: map[string]string{"model": "/missing.gguf"}}
	if status := cache.status(section); status.State != modelpresets.SourceChecking {
		t.Fatalf("first status = %#v", status)
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if status := cache.status(section); status.State == modelpresets.SourceUnavailable {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("background inspection did not populate cache")
}

func TestPresetSourceCacheHandlesNilValues(t *testing.T) {
	if status := newPresetSourceCache().status(modelpresets.Section{}); status.State != modelpresets.SourceUnavailable || status.Error == "" {
		t.Fatalf("status = %#v", status)
	}
}
