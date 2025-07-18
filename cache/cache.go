package cache

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"
)

type Cache[K comparable, V any] struct {
	cache *lru.Cache[K, V]
}

func New[K comparable, V any](maxSize int) *Cache[K, V] {
	c, err := lru.New[K, V](maxSize)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize LRU cache: %s", err.Error()))
	}
	return &Cache[K, V]{cache: c}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	return c.cache.Get(key)
}

func (c *Cache[K, V]) Set(key K, value V) {
	c.cache.Add(key, value)
}
