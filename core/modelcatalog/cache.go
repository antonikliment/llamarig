package modelcatalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"llamarig/platform/filedoc"
)

type catalogCache struct {
	dir string
	ttl time.Duration
}

type catalogCacheEntry struct {
	UpdatedAt string                     `json:"updated_at"`
	Params    normalizedParams           `json:"params"`
	Machine   MachineProfile             `json:"machine"`
	Result    ListResult                 `json:"result"`
	RawList   json.RawMessage            `json:"raw_list,omitempty"`
	RawModels map[string]json.RawMessage `json:"raw_models,omitempty"`
}

func newCatalogCache(dir string, ttl time.Duration) *catalogCache {
	if dir == "" || ttl <= 0 {
		return nil
	}
	return &catalogCache{dir: filepath.Clean(dir), ttl: ttl}
}

func (c *catalogCache) load(params normalizedParams, machine MachineProfile) (catalogCacheEntry, bool, error) {
	if c == nil {
		return catalogCacheEntry{}, false, nil
	}
	data, err := os.ReadFile(c.path(params))
	if os.IsNotExist(err) {
		return catalogCacheEntry{}, false, nil
	}
	if err != nil {
		return catalogCacheEntry{}, false, err
	}
	var entry catalogCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return catalogCacheEntry{}, false, err
	}
	return entry, true, nil
}

func (c *catalogCache) store(params normalizedParams, machine MachineProfile, result ListResult, rawList json.RawMessage, rawModels map[string]json.RawMessage) error {
	if c == nil {
		return nil
	}
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return fmt.Errorf("create catalog cache dir: %w", err)
	}
	entry := catalogCacheEntry{UpdatedAt: time.Now().UTC().Format(time.RFC3339), Params: params, Machine: machine, Result: result, RawList: rawList, RawModels: rawModels}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	if err := filedoc.AtomicCreate(c.path(params), data, 0o600); err != nil {
		return fmt.Errorf("write catalog cache: %w", err)
	}
	return nil
}

func (c *catalogCache) state(entry catalogCacheEntry) CacheState {
	state := CacheState{Hit: true, TTLSeconds: int64(c.ttl.Seconds()), UpdatedAt: entry.UpdatedAt}
	updated, err := time.Parse(time.RFC3339, entry.UpdatedAt)
	if err != nil || time.Since(updated) > c.ttl {
		state.Stale = true
	}
	return state
}

func (c *catalogCache) path(params normalizedParams) string {
	data, _ := json.Marshal(params)
	sum := sha256.Sum256(data)
	return filepath.Join(c.dir, hex.EncodeToString(sum[:])+".json")
}
