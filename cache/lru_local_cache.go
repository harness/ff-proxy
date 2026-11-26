package cache

import (
	"container/list"
	"sync"
	"time"
)

// lruLocalCache: TTL + LRU with optional maxEntries / maxBytes.
// Custom (not stock golang-lru): need joint caps, per-key TTL, Delete, sized costs.
type lruLocalCache struct {
	ttl        time.Duration
	maxEntries int
	maxBytes   int64
	onEvict    func(key string)

	mu    sync.Mutex
	ttls  map[string]time.Time
	items map[string]*list.Element
	lru   *list.List
	bytes int64

	stopJanitor chan struct{}
}

type lruEntry struct {
	key   string
	value interface{}
	size  int64
}

func newLRULocalCache(maxEntries int, maxBytes int64, ttl, cleanupInterval time.Duration, onEvict func(string)) *lruLocalCache {
	c := &lruLocalCache{
		ttl:         ttl,
		maxEntries:  maxEntries,
		maxBytes:    maxBytes,
		onEvict:     onEvict,
		ttls:        make(map[string]time.Time),
		items:       make(map[string]*list.Element),
		lru:         list.New(),
		stopJanitor: make(chan struct{}),
	}
	if cleanupInterval > 0 {
		go c.runJanitor(cleanupInterval)
	}
	return c
}

func (c *lruLocalCache) runJanitor(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			c.DeleteExpired()
		case <-c.stopJanitor:
			return
		}
	}
}

// StopJanitor stops the expiry loop (tests).
func (c *lruLocalCache) StopJanitor() {
	select {
	case <-c.stopJanitor:
	default:
		close(c.stopJanitor)
	}
}

// DeleteExpired drops TTL-expired entries so they stop counting toward caps.
func (c *lruLocalCache) DeleteExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for key, exp := range c.ttls {
		if now.After(exp) {
			if node, ok := c.items[key]; ok {
				c.removeLocked(node)
			}
		}
	}
}

func (c *lruLocalCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, ok := c.items[key]
	if !ok {
		return nil, false
	}
	ent := node.Value.(lruEntry)
	exp, ok := c.ttls[key]
	if !ok || time.Now().After(exp) {
		c.removeLocked(node)
		return nil, false
	}
	c.lru.MoveToFront(node)
	return ent.value, true
}

func (c *lruLocalCache) Set(key string, value interface{}, d time.Duration) {
	_ = c.setRetained(key, value, d)
}

// setRetained stores value when it fits the caps. Returns false when declined
// (oversized vs budget, or still over-budget after eviction) so callers can
// skip retain metrics for keys that never landed in memory.
func (c *lruLocalCache) setRetained(key string, value interface{}, d time.Duration) bool {
	size := approxValueBytes(value)
	// Oversized vs budget: do not cache (Redis refill on next Get).
	if c.maxBytes > 0 && size > c.maxBytes {
		c.mu.Lock()
		if node, ok := c.items[key]; ok {
			c.removeLocked(node)
		}
		c.mu.Unlock()
		return false
	}

	ttl := d
	if ttl <= 0 {
		ttl = c.ttl
	}
	if ttl <= 0 {
		ttl = time.Minute
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if node, ok := c.items[key]; ok {
		old := node.Value.(lruEntry)
		c.bytes -= old.size
		node.Value = lruEntry{key: key, value: value, size: size}
		c.bytes += size
		c.lru.MoveToFront(node)
		c.ttls[key] = time.Now().Add(ttl)
		c.trimLocked()
		return true
	}

	for c.cannotFit(size) && c.lru.Len() > 0 {
		c.removeLocked(c.lru.Back())
	}
	if c.cannotFit(size) {
		return false
	}

	ptr := c.lru.PushFront(lruEntry{key: key, value: value, size: size})
	c.items[key] = ptr
	c.ttls[key] = time.Now().Add(ttl)
	c.bytes += size
	return true
}

func (c *lruLocalCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if node, ok := c.items[key]; ok {
		c.removeLocked(node)
	}
}

func (c *lruLocalCache) ItemCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lru.Len()
}

func (c *lruLocalCache) Bytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bytes
}

func (c *lruLocalCache) cannotFit(incoming int64) bool {
	if c.maxEntries > 0 && c.lru.Len() >= c.maxEntries {
		return true
	}
	if c.maxBytes > 0 && c.bytes+incoming > c.maxBytes {
		return true
	}
	return false
}

func (c *lruLocalCache) trimLocked() {
	for c.maxEntries > 0 && c.lru.Len() > c.maxEntries {
		c.removeLocked(c.lru.Back())
	}
	for c.maxBytes > 0 && c.bytes > c.maxBytes && c.lru.Len() > 0 {
		c.removeLocked(c.lru.Back())
	}
}

func (c *lruLocalCache) removeLocked(node *list.Element) {
	ent := node.Value.(lruEntry)
	c.lru.Remove(node)
	delete(c.items, ent.key)
	delete(c.ttls, ent.key)
	c.bytes -= ent.size
	if c.bytes < 0 {
		c.bytes = 0
	}
	if c.onEvict != nil {
		c.onEvict(ent.key)
	}
}

func approxValueBytes(v interface{}) int64 {
	switch x := v.(type) {
	case hashCacheEntry:
		if x.size > 0 {
			return x.size + int64(len(x.hash)) + 32
		}
		return 256
	case []byte:
		return int64(len(x) + 24)
	case string:
		return int64(len(x) + 24)
	default:
		return 256
	}
}
