package tx

import (
	"log/slog"
	"sync"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/initia-labs/rollytics/indexer/config"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

const SubmoduleName = "tx"

var _ indexertypes.Submodule = &TxSubmodule{}

type TxSubmodule struct {
	logger   *slog.Logger
	cfg      *config.Config
	cdc      codec.Codec
	amino    *codec.LegacyAmino
	evmTxMap map[int64][]types.EvmTx
	mtx      sync.Mutex
}

func New(logger *slog.Logger, cfg *config.Config, cdc codec.Codec, amino *codec.LegacyAmino) *TxSubmodule {
	return &TxSubmodule{
		logger:   logger.With("submodule", SubmoduleName),
		cfg:      cfg,
		cdc:      cdc,
		amino:    amino,
		evmTxMap: make(map[int64][]types.EvmTx),
	}
}

func (sub *TxSubmodule) Name() string {
	return SubmoduleName
}

func (sub *TxSubmodule) Prepare(block indexertypes.ScrapedBlock) error {
	if err := sub.prepare(block); err != nil {
		sub.logger.Error("failed to prepare data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}

func (sub *TxSubmodule) Collect(block indexertypes.ScrapedBlock, tx *gorm.DB) error {
	if err := sub.collect(block, tx); err != nil {
		sub.logger.Error("failed to collect data", slog.Int64("height", block.Height), slog.Any("error", err))
		return err
	}

	return nil
}
