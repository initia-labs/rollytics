package block

import (
	"log/slog"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/initia-labs/rollytics/indexer/types"
	"gorm.io/gorm"
)

const submoduleName = "block"

var _ types.Submodule = BlockSubmodule{}

type BlockSubmodule struct {
	logger   *slog.Logger
	txConfig client.TxConfig
}

func New(logger *slog.Logger, txConfig client.TxConfig) *BlockSubmodule {
	return &BlockSubmodule{
		logger:   logger.With("submodule", submoduleName),
		txConfig: txConfig,
	}
}

func (sub BlockSubmodule) Collect(block types.ScrappedBlock, tx *gorm.DB) error {
	return sub.collect(block, tx)
}
