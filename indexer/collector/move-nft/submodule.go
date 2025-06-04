package move_nft

import (
	"log/slog"
	"sync"

	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/types"
	"gorm.io/gorm"
)

const SubmoduleName = "move-nft"

var _ types.Submodule = &MoveNftSubmodule{}

type MoveNftSubmodule struct {
	logger *slog.Logger
	cfg    *config.Config
	cache  map[int64]CacheData
	mtx    sync.Mutex
}

func New(logger *slog.Logger, cfg *config.Config) *MoveNftSubmodule {
	return &MoveNftSubmodule{
		logger: logger.With("submodule", SubmoduleName),
		cfg:    cfg,
		cache:  make(map[int64]CacheData),
	}
}

func (sub *MoveNftSubmodule) Name() string {
	return SubmoduleName
}

func (sub *MoveNftSubmodule) Prepare(block types.ScrappedBlock) error {
	if err := sub.prepare(block); err != nil {
		sub.logger.Error("failed to prepare data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}

func (sub *MoveNftSubmodule) Collect(block types.ScrappedBlock, tx *gorm.DB) error {
	if err := sub.collect(block, tx); err != nil {
		sub.logger.Error("failed to collect data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}
