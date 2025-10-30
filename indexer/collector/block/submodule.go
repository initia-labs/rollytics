package block

import (
	"log/slog"

	"github.com/cosmos/cosmos-sdk/codec"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/types"
)

const SubmoduleName = "block"

var _ types.Submodule = &BlockSubmodule{}

type BlockSubmodule struct {
	logger *slog.Logger
	cdc    codec.Codec
	cfg    *config.Config
}

func New(logger *slog.Logger, cfg *config.Config, cdc codec.Codec) *BlockSubmodule {
	return &BlockSubmodule{
		logger: logger.With("submodule", SubmoduleName),
		cdc:    cdc,
		cfg:    cfg,
	}
}

func (sub *BlockSubmodule) Name() string {
	return SubmoduleName
}

func (sub *BlockSubmodule) Prepare(block types.ScrapedBlock) error {
	return nil
}

func (sub *BlockSubmodule) Collect(block types.ScrapedBlock, tx *gorm.DB) error {
	if err := sub.collect(block, tx); err != nil {
		sub.logger.Error("failed to collect data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}
