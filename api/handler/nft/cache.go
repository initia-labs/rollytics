package nft

import (
	"github.com/initia-labs/rollytics/cache"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

// cache for collection data
var collectionCache = cache.New[string, *types.CollectedNftCollection](100)

func getCollection(database *orm.Database, collectionAddr string) (*types.CollectedNftCollection, error) {
	cached, ok := collectionCache.Get(collectionAddr)
	if ok {
		return cached, nil
	}

	var collection types.CollectedNftCollection
	if err := database.Where("addr = ?", collectionAddr).First(&collection).Error; err != nil {
		return &collection, err
	}

	collectionCache.Set(collectionAddr, &collection)

	return &collection, nil
}
