package leaderboard

import (
	"fmt"
	"sync"
	"time"
)

// ListCache caches the result of repo.List(goal, limit).
// Per-user lookups (ListMine) are intentionally not cached.
// Submit invalidates only the affected goal — other goals stay warm.
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

// InvalidateGoal drops every cached limit-variant of the given goal.
// Other goals are untouched, avoiding cache stampede across unrelated goals.
func (c *ListCache) InvalidateGoal(goal int) {
	prefix := fmt.Sprintf("%d:", goal)
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.entries {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.entries, k)
		}
	}
}
