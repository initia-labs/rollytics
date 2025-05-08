package tx

import (
	"log/slog"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/initia-labs/rollytics/indexer/types"
	"gorm.io/gorm"
)

const SubmoduleName = "tx"

var _ types.Submodule = TxSubmodule{}

type TxSubmodule struct {
	logger   *slog.Logger
	txConfig client.TxConfig
}

func New(logger *slog.Logger, txConfig client.TxConfig) *TxSubmodule {
	return &TxSubmodule{
		logger:   logger.With("submodule", SubmoduleName),
		txConfig: txConfig,
	}
}

func (sub TxSubmodule) Name() string {
	return SubmoduleName
}

func (sub TxSubmodule) Prepare(block types.ScrappedBlock) error {
	return nil
}

func (sub TxSubmodule) Collect(block types.ScrappedBlock, tx *gorm.DB) error {
	if err := sub.collect(block, tx); err != nil {
		sub.logger.Error("failed to collect data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}
