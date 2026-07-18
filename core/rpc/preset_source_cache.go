package rpc

import (
	"sync"
	"time"

	"llamarig/core/modelpresets"
)

const presetSourceCacheTTL = 5 * time.Second

type presetSourceCacheEntry struct {
	key        string
	status     modelpresets.SourceStatus
	checkedAt  time.Time
	refreshing bool
}

type presetSourceCache struct {
	mu      sync.Mutex
	entries map[string]*presetSourceCacheEntry
	inspect func(modelpresets.Section) modelpresets.SourceStatus
	now     func() time.Time
}

func newPresetSourceCache() *presetSourceCache {
	return &presetSourceCache{entries: map[string]*presetSourceCacheEntry{}, inspect: modelpresets.InspectSource, now: time.Now}
}

func (c *presetSourceCache) status(section modelpresets.Section) modelpresets.SourceStatus {
	if section.Values == nil {
		return modelpresets.SourceStatus{State: modelpresets.SourceUnavailable, Error: "preset has no configured values"}
	}
	key := section.Values["model"] + "\x00" + section.Values["models-dir"]
	section = modelpresets.Section{Name: section.Name, Values: map[string]string{"model": section.Values["model"], "models-dir": section.Values["models-dir"]}}
	c.mu.Lock()
	entry := c.entries[section.Name]
	if entry != nil && entry.key == key {
		status := entry.status
		if !entry.refreshing && c.now().Sub(entry.checkedAt) >= presetSourceCacheTTL {
			entry.refreshing = true
			go c.refresh(section, key)
		}
		c.mu.Unlock()
		return status
	}
	c.entries[section.Name] = &presetSourceCacheEntry{key: key, status: modelpresets.SourceStatus{State: modelpresets.SourceChecking}, refreshing: true}
	c.mu.Unlock()
	go c.refresh(section, key)
	return modelpresets.SourceStatus{State: modelpresets.SourceChecking}
}

func (c *presetSourceCache) refresh(section modelpresets.Section, key string) {
	status := c.inspect(section)
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry := c.entries[section.Name]; entry != nil && entry.key == key {
		entry.status, entry.checkedAt, entry.refreshing = status, c.now(), false
	}
}
