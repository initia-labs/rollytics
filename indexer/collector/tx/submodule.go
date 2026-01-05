package tx

import (
	"context"
	"log/slog"
	"sync"

	"github.com/cosmos/cosmos-sdk/codec"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/util/querier"
)

const SubmoduleName = "tx"

var _ indexertypes.Submodule = &TxSubmodule{}

type TxSubmodule struct {
	logger  *slog.Logger
	cfg     *config.Config
	cdc     codec.Codec
	cache   map[int64]CacheData
	mtx     sync.Mutex
	querier *querier.Querier
}

func New(logger *slog.Logger, cfg *config.Config, cdc codec.Codec) *TxSubmodule {
	return &TxSubmodule{
		logger:  logger.With("submodule", SubmoduleName),
		cfg:     cfg,
		cdc:     cdc,
		cache:   make(map[int64]CacheData),
		querier: querier.NewQuerier(cfg.GetChainConfig()),
	}
}

func (sub *TxSubmodule) Name() string {
	return SubmoduleName
}

func (sub *TxSubmodule) Prepare(ctx context.Context, block indexertypes.ScrapedBlock) error {
	if err := sub.prepare(ctx, block); err != nil {
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
