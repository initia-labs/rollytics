package tx

import "sync"

type FAStoreCache struct {
	m   map[string]string // store addr -> owner
	mtx sync.RWMutex
}

func NewFAStoreCache() *FAStoreCache {
	return &FAStoreCache{
		m: make(map[string]string),
	}
}

func (cache *FAStoreCache) Get(storeAddr string) (string, bool) {
	cache.mtx.RLock()
	defer cache.mtx.RUnlock()
	val, ok := cache.m[storeAddr]
	return val, ok
}

func (cache *FAStoreCache) Set(storeAddr, owner string) {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	if len(cache.m) >= 10000 {
		cache.m = make(map[string]string)
	}

	cache.m[storeAddr] = owner
}
