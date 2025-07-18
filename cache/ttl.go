package cache

import (
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

type TTLCache[K comparable, V any] struct {
	cache *expirable.LRU[K, V]
}

func NewTTL[K comparable, V any](maxSize int, ttl time.Duration) *TTLCache[K, V] {
	c := expirable.NewLRU[K, V](maxSize, nil, ttl)
	return &TTLCache[K, V]{cache: c}
}

func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	return c.cache.Get(key)
}

func (c *TTLCache[K, V]) Set(key K, value V) {
	c.cache.Add(key, value)
}
