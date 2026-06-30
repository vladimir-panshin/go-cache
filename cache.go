package cache

import (
	"sync"
	"time"
)

type cacheItem[V any] struct {
	value     V
	expiresAt time.Time
}

func (item cacheItem[V]) isExpired() bool {
	return !item.expiresAt.IsZero() && time.Now().After(item.expiresAt)
}

type Cache[V any] struct {
	mu    sync.RWMutex
	items map[string]cacheItem[V]

	stop      chan struct{}
	closeOnce sync.Once
}

func New[V any](cleanupInterval time.Duration) *Cache[V] {
	c := &Cache[V]{
		items: make(map[string]cacheItem[V]),
		stop:  make(chan struct{}),
	}

	if cleanupInterval > 0 {
		go c.startGC(cleanupInterval)
	}

	return c
}

func (c *Cache[V]) Set(key string, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	c.items[key] = cacheItem[V]{
		value:     value,
		expiresAt: expiresAt,
	}
}

func (c *Cache[V]) SetNX(key string, value V, ttl time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, exists := c.items[key]; exists && !item.isExpired() {
		return false
	}

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	c.items[key] = cacheItem[V]{
		value:     value,
		expiresAt: expiresAt,
	}

	return true
}

func (c *Cache[V]) Get(key string) (V, bool) {
	item, ok := c.getItem(key)
	if !ok {
		var zero V
		return zero, false
	}

	return item.value, true
}

func (c *Cache[V]) TTL(key string) (time.Duration, bool) {
	item, ok := c.getItem(key)
	if !ok {
		return 0, false
	}

	if item.expiresAt.IsZero() {
		return -1, true
	}

	return time.Until(item.expiresAt), true
}

func (c *Cache[V]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

func (c *Cache[V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}

func (c *Cache[V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]cacheItem[V])
}

func (c *Cache[V]) getItem(key string) (cacheItem[V], bool) {
	c.mu.RLock()

	item, exists := c.items[key]

	c.mu.RUnlock()

	if !exists {
		return cacheItem[V]{}, false
	}

	if !item.isExpired() {
		return item, true
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists = c.items[key]
	if !exists {
		return cacheItem[V]{}, false
	}

	if item.isExpired() {
		delete(c.items, key)
		return cacheItem[V]{}, false
	}

	return item, true
}

func (c *Cache[V]) startGC(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()

			for key, item := range c.items {
				if item.isExpired() {
					delete(c.items, key)
				}
			}

			c.mu.Unlock()

		case <-c.stop:
			return
		}
	}
}

func (c *Cache[V]) Close() {
	c.closeOnce.Do(func() {
		close(c.stop)
	})
}
