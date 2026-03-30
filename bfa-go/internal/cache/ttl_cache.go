package cache

import (
	"sync"
	"time"
)

type entry[T any] struct {
	value     T
	expiresAt time.Time
}

type TTLCache[T any] struct {
	mu    sync.RWMutex
	items map[string]entry[T]
	now   func() time.Time
}

func New[T any]() *TTLCache[T] {
	return &TTLCache[T]{
		items: make(map[string]entry[T]),
		now:   time.Now,
	}
}

func (c *TTLCache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[key]
	if !ok || c.now().After(item.expiresAt) {
		var zero T
		return zero, false
	}
	return item.value, true
}

func (c *TTLCache[T]) Set(key string, value T, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = entry[T]{value: value, expiresAt: c.now().Add(ttl)}
}
