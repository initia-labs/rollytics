package tx

import (
	"log/slog"
	"sync"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/types"
	"gorm.io/gorm"
)

const SubmoduleName = "tx"

var _ types.Submodule = &TxSubmodule{}

type TxSubmodule struct {
	logger   *slog.Logger
	cfg      *config.Config
	txConfig client.TxConfig
	cdc      codec.Codec
	evmTxMap map[int64][]EvmTx
	mtx      sync.Mutex
}

func New(logger *slog.Logger, cfg *config.Config, txConfig client.TxConfig, cdc codec.Codec) *TxSubmodule {
	return &TxSubmodule{
		logger:   logger.With("submodule", SubmoduleName),
		cfg:      cfg,
		txConfig: txConfig,
		cdc:      cdc,
		evmTxMap: make(map[int64][]EvmTx),
	}
}

func (sub *TxSubmodule) Name() string {
	return SubmoduleName
}

func (sub *TxSubmodule) Prepare(block types.ScrapedBlock) error {
	if err := sub.prepare(block); err != nil {
		sub.logger.Error("failed to prepare data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}

func (sub *TxSubmodule) Collect(block types.ScrapedBlock, tx *gorm.DB) error {
	if err := sub.collect(block, tx); err != nil {
		sub.logger.Error("failed to collect data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}
