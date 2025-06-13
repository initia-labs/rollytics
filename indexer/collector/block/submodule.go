package block

import (
	"log/slog"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/initia-labs/rollytics/indexer/types"
	"gorm.io/gorm"
)

const SubmoduleName = "block"

var _ types.Submodule = &BlockSubmodule{}

type BlockSubmodule struct {
	logger *slog.Logger
	cdc    codec.Codec
}

func New(logger *slog.Logger, cdc codec.Codec) *BlockSubmodule {
	return &BlockSubmodule{
		logger: logger.With("submodule", SubmoduleName),
		cdc:    cdc,
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
