package tx

import (
	"log/slog"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/initia-labs/rollytics/indexer/types"
	"gorm.io/gorm"
)

const submoduleName = "tx"

var _ types.Submodule = TxSubmodule{}

type TxSubmodule struct {
	logger   *slog.Logger
	txConfig client.TxConfig
}

func New(logger *slog.Logger, txConfig client.TxConfig) *TxSubmodule {
	return &TxSubmodule{
		logger:   logger.With("submodule", submoduleName),
		txConfig: txConfig,
	}
}

func (sub TxSubmodule) Collect(block types.ScrappedBlock, tx *gorm.DB) error {
	if err := sub.collectTx(block, tx); err != nil {
		return err
	}

	if err := sub.collectAccountTx(block, tx); err != nil {
		return err
	}

	return nil
}
