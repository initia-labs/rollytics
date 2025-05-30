package wasm_nft

import (
	"log/slog"
	"sync"

	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/types"
	"gorm.io/gorm"
)

const SubmoduleName = "wasm-nft"

var _ types.Submodule = &WasmNftSubmodule{}

type WasmNftSubmodule struct {
	logger  *slog.Logger
	cfg     *config.Config
	dataMap map[int64]CacheData
	mtx     sync.Mutex
}

func New(logger *slog.Logger, cfg *config.Config) *WasmNftSubmodule {
	return &WasmNftSubmodule{
		logger:  logger.With("submodule", SubmoduleName),
		cfg:     cfg,
		dataMap: make(map[int64]CacheData),
	}
}

func (sub *WasmNftSubmodule) Name() string {
	return SubmoduleName
}

func (sub *WasmNftSubmodule) Prepare(block types.ScrappedBlock) error {
	if err := sub.prepare(block); err != nil {
		sub.logger.Error("failed to prepare data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}

func (sub *WasmNftSubmodule) Collect(block types.ScrappedBlock, tx *gorm.DB) error {
	if err := sub.collect(block, tx); err != nil {
		sub.logger.Error("failed to collect data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}
