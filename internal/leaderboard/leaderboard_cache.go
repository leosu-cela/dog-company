package leaderboard

import (
	"fmt"
	"sync"
	"time"
)

// ListCache caches the result of repo.List(goal, limit).
// my_best is per-user and therefore intentionally not cached.
// Submit invalidates the whole cache (cheap: small map, rare writes).
type ListCache struct {
	mu      sync.RWMutex
	entries map[string]cachedList
	ttl     time.Duration
}

type cachedList struct {
	data      []Entry
	expiresAt time.Time
}

func NewListCache(ttl time.Duration) *ListCache {
	return &ListCache{
		entries: make(map[string]cachedList),
		ttl:     ttl,
	}
}

func (c *ListCache) key(goal, limit int) string {
	return fmt.Sprintf("%d:%d", goal, limit)
}

func (c *ListCache) Get(goal, limit int) ([]Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[c.key(goal, limit)]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.data, true
}

func (c *ListCache) Set(goal, limit int, data []Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[c.key(goal, limit)] = cachedList{
		data:      data,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *ListCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]cachedList)
}
