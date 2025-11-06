package nft

import (
	"database/sql"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/cache"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
	"github.com/initia-labs/rollytics/util/common-handler/common"
)

type cachedCol struct {
	types.CollectedNftCollection `gorm:"embedded"`
	NormalizedName               string `gorm:"-"`
	NormalizedOriginName         string `gorm:"-"`
} // ordered by height ASC

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

func initCollectionCache(database *orm.Database, cfg *config.Config) {
	collectionCacheOnce.Do(func() {
		cacheSize := cfg.GetCacheSize()
		ttl := cfg.GetCacheTTL()
		collectionCacheByAddr = cache.NewTTL[string, *types.CollectedNftCollection](cacheSize, ttl)
		tryUpdateCollectionCache(database.DB, cfg)
	})
}

func getCollectionByAddr(database *orm.Database, collectionAddr string) (*types.CollectedNftCollection, error) {
	cached, ok := collectionCacheByAddr.Get(collectionAddr)
	if ok {
		return cached, nil
	}

	collectionAddrBytes, err := util.HexToBytes(collectionAddr)
	if err != nil {
		return nil, err
	}

	var collection types.CollectedNftCollection
	if err := database.Model(&types.CollectedNftCollection{}).Where("addr = ?", collectionAddrBytes).First(&collection).Error; err != nil {
		return &collection, err
	}

	collectionCacheByAddr.Set(collectionAddr, &collection)

	return &collection, nil
}

func getCollectionByNameFromCache(tx *gorm.DB, cfg *config.Config, name string, pagination *common.Pagination) ([]types.CollectedNftCollection, int64) {
	tryUpdateCollectionCache(tx, cfg)

	name = strings.ToLower(sanitizer.ReplaceAllString(name, ""))
	var results []types.CollectedNftCollection
	cacheMtx.RLock()
	for _, c := range collectionCacheByName {
		if strings.Contains(c.NormalizedName, name) || strings.Contains(c.NormalizedOriginName, name) {
			results = append(results, c.CollectedNftCollection)
		}
	}
	if pagination.Order == common.OrderDesc {
		slices.Reverse(results)
	}
	cacheMtx.RUnlock()

	// apply offset and limit
	total := len(results)
	if pagination.Offset >= total {
		return nil, int64(total)
	} else {
		results = results[pagination.Offset:]
	}

	if pagination.Limit > 0 && len(results) > pagination.Limit {
		results = results[:pagination.Limit]
	}

	return results, int64(total)
}

// getCollectionByName is a wrapper that accepts a transaction but delegates to the cache-based implementation
func getCollectionByName(tx *gorm.DB, cfg *config.Config, name string, pagination *common.Pagination) ([]types.CollectedNftCollection, int64) {
	// The cache implementation works with cached data and uses readonly transaction for updates
	return getCollectionByNameFromCache(tx, cfg, name, pagination)
}

func tryUpdateCollectionCache(db *gorm.DB, cfg *config.Config) {
	if time.Since(time.Unix(0, lastUpdatedTime.Load())) < cfg.GetPollingInterval() {
		return
	}
	// check if already updated
	if !updating.CompareAndSwap(false, true) {
		return
	}

	// Start our own ReadOnly transaction for cache update
	updateTx := db.Begin(&sql.TxOptions{ReadOnly: true})
	defer updateTx.Rollback()

	updateCollectionCache(updateTx)
}

func updateCollectionCache(tx *gorm.DB) {
	defer updating.Store(false)

	var cols []cachedCol
	err := tx.Model(&types.CollectedNftCollection{}).
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
