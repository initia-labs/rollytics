package block

import (
	"log/slog"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/initia-labs/rollytics/indexer/types"
	"gorm.io/gorm"
)

const SubmoduleName = "block"

var _ types.Submodule = BlockSubmodule{}

type BlockSubmodule struct {
	logger   *slog.Logger
	txConfig client.TxConfig
}

func New(logger *slog.Logger, txConfig client.TxConfig) *BlockSubmodule {
	return &BlockSubmodule{
		logger:   logger.With("submodule", SubmoduleName),
		txConfig: txConfig,
	}
}

func (sub BlockSubmodule) Name() string {
	return SubmoduleName
}

func (sub BlockSubmodule) Prepare(block types.ScrappedBlock) error {
	return nil
}

func (sub BlockSubmodule) Collect(block types.ScrappedBlock, tx *gorm.DB) error {
	return sub.collect(block, tx)
}
