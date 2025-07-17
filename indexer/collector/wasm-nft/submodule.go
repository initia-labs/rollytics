package wasm_nft

import (
	"log/slog"
	"sync"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/cache"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/types"
)

const SubmoduleName = "wasm-nft"

var _ types.Submodule = &WasmNftSubmodule{}

type WasmNftSubmodule struct {
	logger    *slog.Logger
	cfg       *config.Config
	cache     map[int64]CacheData
	blacklist *cache.Cache[string, struct{}]
	mtx       sync.Mutex
}

func New(logger *slog.Logger, cfg *config.Config) *WasmNftSubmodule {
	return &WasmNftSubmodule{
		logger:    logger.With("submodule", SubmoduleName),
		cfg:       cfg,
		cache:     make(map[int64]CacheData),
		blacklist: cache.New[string, struct{}](1000),
	}
}

func (sub *WasmNftSubmodule) Name() string {
	return SubmoduleName
}

func (sub *WasmNftSubmodule) Prepare(block types.ScrapedBlock) error {
	if err := sub.prepare(block); err != nil {
		sub.logger.Error("failed to prepare data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}

func (sub *WasmNftSubmodule) Collect(block types.ScrapedBlock, tx *gorm.DB) error {
	if err := sub.collect(block, tx); err != nil {
		sub.logger.Error("failed to collect data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}

func (sub *WasmNftSubmodule) AddToBlacklist(addr string) {
	sub.blacklist.Set(addr, struct{}{})
}

func (sub *WasmNftSubmodule) IsBlacklisted(addr string) bool {
	_, found := sub.blacklist.Get(addr)
	return found
}
