package nft

import (
	"log/slog"

	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/types"
	"gorm.io/gorm"
)

const SubmoduleName = "nft"

var _ types.Submodule = NftSubmodule{}

type NftSubmodule struct {
	logger  *slog.Logger
	cfg     *config.Config
	dataMap map[int64]CacheData
}

func New(logger *slog.Logger, cfg *config.Config) *NftSubmodule {
	return &NftSubmodule{
		logger:  logger.With("submodule", SubmoduleName),
		cfg:     cfg,
		dataMap: make(map[int64]CacheData),
	}
}

func (sub NftSubmodule) Name() string {
	return SubmoduleName
}

func (sub NftSubmodule) Prepare(block types.ScrappedBlock) error {
	if err := sub.prepare(block); err != nil {
		sub.logger.Error("failed to prepare data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}

func (sub NftSubmodule) Collect(block types.ScrappedBlock, tx *gorm.DB) error {
	if err := sub.collect(block, tx); err != nil {
		sub.logger.Error("failed to collect data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}
