package nft

import (
	"github.com/initia-labs/rollytics/cache"
	"github.com/initia-labs/rollytics/orm"
	dbtypes "github.com/initia-labs/rollytics/types"
)

// cache for collection data
var (
	collectionCache = cache.New[string, *dbtypes.CollectedNftCollection](100)
)

func getCollection(database *orm.Database, collectionAddr string) (*dbtypes.CollectedNftCollection, error) {
	cached, ok := collectionCache.Get(collectionAddr)
	if ok {
		return cached, nil
	}

	var collection dbtypes.CollectedNftCollection
	if res := database.Where("addr = ?", collectionAddr).First(&collection); res.Error != nil {
		return &collection, res.Error
	}

	collectionCache.Set(collectionAddr, &collection)

	return &collection, nil
}
