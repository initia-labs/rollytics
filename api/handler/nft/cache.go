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

// cache for collection data
var addr2Collections = cache.NewTTL[string, *types.CollectedNftCollection](100, 10*time.Minute)

func getCollectionByAddr(database *orm.Database, collectionAddr string) (*types.CollectedNftCollection, error) {
	cached, ok := addr2Collections.Get(collectionAddr)
	if ok {
		return cached, nil
	}

	var collection types.CollectedNftCollection
	if err := database.Where("addr = ?", collectionAddr).First(&collection).Error; err != nil {
		return &collection, err
	}

	addr2Collections.Set(collectionAddr, &collection)

	return &collection, nil
}

type cachedCol struct {
	types.CollectedNftCollection `gorm:"embedded"`
	NormalizedName               string `gorm:"-"`
	NormalizedOriginName         string `gorm:"-"`
}

var (
	collectionCache []cachedCol // ordered by height ASC
	cacheMu         sync.RWMutex

	initOnce          sync.Once
	lastFetchedHeight int64
	lastRefreshTime   atomic.Int64
	refreshing        atomic.Bool

	sanitizer = regexp.MustCompile(`[^\p{L}\p{M}\p{N}]+`)
)

func getCollectionByName(db *orm.Database, name string, pagination *common.Pagination) ([]types.CollectedNftCollection, int64, error) {
	tryRefreshCache(db)

	name = strings.ToLower(sanitizer.ReplaceAllString(name, ""))
	var results []types.CollectedNftCollection
	cacheMu.RLock()
	for _, c := range collectionCache {
		if strings.Contains(c.NormalizedName, name) || strings.Contains(c.NormalizedOriginName, name) {
			results = append(results, c.CollectedNftCollection)
		}
	}
	if pagination.Order == "DESC" {
		slices.Reverse(results)
	}
	cacheMu.RUnlock()

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

const refreshInterval = 3 * time.Second

func refreshCache(db *orm.Database) {
	defer refreshing.Store(false)

	var cols []cachedCol
	err := db.Model(&types.CollectedNftCollection{}).
		Where("height > ?", lastFetchedHeight).
		Order("height ASC").
		Find(&cols).Error
	if err != nil || len(cols) == 0 {
		return
	}

	for i := range cols {
		cols[i].NormalizedName = strings.ToLower(sanitizer.ReplaceAllString(cols[i].Name, ""))
		cols[i].NormalizedOriginName = strings.ToLower(sanitizer.ReplaceAllString(cols[i].OriginName, ""))
	}

	cacheMu.Lock()
	collectionCache = append(collectionCache, cols...)
	lastFetchedHeight = cols[len(cols)-1].Height
	cacheMu.Unlock()

	lastRefreshTime.Store(time.Now().UnixNano())
}

func tryRefreshCache(db *orm.Database) {
	if time.Since(time.Unix(0, lastRefreshTime.Load())) < refreshInterval {
		return
	}
	// check if already refreshing
	if !refreshing.CompareAndSwap(false, true) {
		return
	}

	go refreshCache(db)
}
