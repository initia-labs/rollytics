package cache

import "sync"

type BoundedCache[K comparable, V any] struct {
	m       map[K]V
	mtx     sync.RWMutex
	maxSize int
}

func New[K comparable, V any](maxSize int) *BoundedCache[K, V] {
	return &BoundedCache[K, V]{
		m:       make(map[K]V),
		maxSize: maxSize,
	}
}

func (cache *BoundedCache[K, V]) Get(key K) (V, bool) {
	cache.mtx.RLock()
	defer cache.mtx.RUnlock()
	val, ok := cache.m[key]
	return val, ok
}

func (cache *BoundedCache[K, V]) Set(key K, value V) {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	if len(cache.m) >= cache.maxSize {
		cache.m = make(map[K]V)
	}

	cache.m[key] = value
}
