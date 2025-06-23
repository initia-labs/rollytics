package nft

import (
	"sync"

	"github.com/initia-labs/rollytics/orm"
	dbtypes "github.com/initia-labs/rollytics/types"
)

// cache for validator responses
var (
	collectionCache     = make(map[string]*dbtypes.CollectedNftCollection)
	collectionCacheLock sync.RWMutex
)

func getCollection(database *orm.Database, collectionAddr string) (*dbtypes.CollectedNftCollection, error) {
	collectionCacheLock.RLock()
	cached, ok := collectionCache[collectionAddr]
	collectionCacheLock.RUnlock()
	if ok {
		return cached, nil
	}

	var collection dbtypes.CollectedNftCollection
	database.Where("addr = ?", collectionAddr).First(&collection)

	collectionCacheLock.Lock()
	collectionCache[collectionAddr] = &collection
	collectionCacheLock.Unlock()

	return &collection, nil
}
