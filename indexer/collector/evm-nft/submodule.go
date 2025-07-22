package evm_nft

import (
	"log/slog"
	"sync"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/cache"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/types"
)

const SubmoduleName = "evm-nft"

var _ types.Submodule = &EvmNftSubmodule{}

type EvmNftSubmodule struct {
	logger    *slog.Logger
	cfg       *config.Config
	cache     map[int64]CacheData
	blacklist *cache.Cache[string, interface{}]
	mtx       sync.Mutex
}

func New(logger *slog.Logger, cfg *config.Config) *EvmNftSubmodule {
	cacheSize := cfg.GetCacheSize()
	return &EvmNftSubmodule{
		logger:    logger.With("submodule", SubmoduleName),
		cfg:       cfg,
		cache:     make(map[int64]CacheData),
		blacklist: cache.New[string, interface{}](cacheSize),
	}
}

func (sub *EvmNftSubmodule) Name() string {
	return SubmoduleName
}

func (sub *EvmNftSubmodule) Prepare(block types.ScrapedBlock) error {
	if err := sub.prepare(block); err != nil {
		sub.logger.Error("failed to prepare data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}

func (sub *EvmNftSubmodule) Collect(block types.ScrapedBlock, tx *gorm.DB) error {
	if err := sub.collect(block, tx); err != nil {
		sub.logger.Error("failed to collect data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}

func (sub *EvmNftSubmodule) AddToBlacklist(addr string) {
	sub.blacklist.Set(addr, nil)
}

func (sub *EvmNftSubmodule) IsBlacklisted(addr string) bool {
	_, found := sub.blacklist.Get(addr)
	return found
}
