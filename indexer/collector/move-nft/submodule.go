package move_nft

import (
	"context"
	"log/slog"
	"sync"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/util/querier"
)

const SubmoduleName = "move-nft"

var _ types.Submodule = &MoveNftSubmodule{}

type MoveNftSubmodule struct {
	logger  *slog.Logger
	cfg     *config.Config
	cache   map[int64]CacheData
	mtx     sync.Mutex
	querier *querier.Querier
}

func New(logger *slog.Logger, cfg *config.Config) *MoveNftSubmodule {
	return &MoveNftSubmodule{
		logger:  logger.With("submodule", SubmoduleName),
		cfg:     cfg,
		cache:   make(map[int64]CacheData),
		querier: querier.NewQuerier(cfg.GetChainConfig()),
	}
}

func (sub *MoveNftSubmodule) Name() string {
	return SubmoduleName
}

func (sub *MoveNftSubmodule) Prepare(ctx context.Context, block types.ScrapedBlock) error {
	if err := sub.prepare(ctx, block); err != nil {
		sub.logger.Error("failed to prepare data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}

func (sub *MoveNftSubmodule) Collect(block types.ScrapedBlock, tx *gorm.DB) error {
	if err := sub.collect(block, tx); err != nil {
		sub.logger.Error("failed to collect data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}
