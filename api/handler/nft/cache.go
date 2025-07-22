package nft

import (
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/cache"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

type cachedCol struct {
	types.CollectedNftCollection `gorm:"embedded"`
	NormalizedName               string `gorm:"-"`
	NormalizedOriginName         string `gorm:"-"`
} // ordered by height ASC

const updatingInterval = 3 * time.Second

// cache for collection data
var (
	collectionCacheOnce sync.Once
	// addr
	collectionCacheByAddr *cache.TTLCache[string, *types.CollectedNftCollection]
	// name
	collectionCacheByName []cachedCol
	cacheMtx              sync.RWMutex
	lastUpdatedHeight     int64
	lastUpdatedTime       atomic.Int64
	updating              atomic.Bool

	sanitizer = regexp.MustCompile(`[^\p{L}\p{M}\p{N}]+`)
)

func getCollectionByAddr(database *orm.Database, collectionAddr string) (*types.CollectedNftCollection, error) {
	cached, ok := collectionCacheByAddr.Get(collectionAddr)
	if ok {
		return cached, nil
	}

	var collection types.CollectedNftCollection
	if err := database.Model(&types.CollectedNftCollection{}).Where("addr = ?", collectionAddr).First(&collection).Error; err != nil {
		return &collection, err
	}

	collectionCacheByAddr.Set(collectionAddr, &collection)

	return &collection, nil
}

func getCollectionByName(db *orm.Database, name string, pagination *common.Pagination) ([]types.CollectedNftCollection, int64, error) {
	tryUpdateCollectionCache(db)

	name = strings.ToLower(sanitizer.ReplaceAllString(name, ""))
	var results []types.CollectedNftCollection
	cacheMtx.RLock()
	for _, c := range collectionCacheByName {
		if strings.Contains(c.NormalizedName, name) || strings.Contains(c.NormalizedOriginName, name) {
			results = append(results, c.CollectedNftCollection)
		}
	}
	if pagination.Order == common.DefaultPaginationOrderDesc {
		slices.Reverse(results)
	}
	cacheMtx.RUnlock()

	// apply offset and limit
	total := len(results)
	if pagination.Offset >= total {
		return nil, int64(total), nil
	} else {
		results = results[pagination.Offset:]
	}
	if pagination.Limit > 0 && len(results) > pagination.Limit {
		results = results[:pagination.Limit]
	}
	return results, int64(total), nil
}

func tryUpdateCollectionCache(db *orm.Database) {
	if time.Since(time.Unix(0, lastUpdatedTime.Load())) < updatingInterval {
		return
	}
	// check if already updated
	if !updating.CompareAndSwap(false, true) {
		return
	}

	updateCollectionCache(db)
}

func updateCollectionCache(db *orm.Database) {
	defer updating.Store(false)

	var cols []cachedCol
	err := db.Model(&types.CollectedNftCollection{}).
		Where("height > ?", lastUpdatedHeight).
		Order("height ASC").
		Find(&cols).Error
	if err != nil || len(cols) == 0 {
		return
	}

	for i := range cols {
		cols[i].NormalizedName = strings.ToLower(sanitizer.ReplaceAllString(cols[i].Name, ""))
		cols[i].NormalizedOriginName = strings.ToLower(sanitizer.ReplaceAllString(cols[i].OriginName, ""))
	}

	cacheMtx.Lock()
	defer cacheMtx.Unlock()
	collectionCacheByName = append(collectionCacheByName, cols...)
	lastUpdatedHeight = cols[len(cols)-1].Height
	lastUpdatedTime.Store(time.Now().UnixNano())
}
