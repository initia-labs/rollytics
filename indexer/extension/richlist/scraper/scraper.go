package scraper

import (
	"context"
	"fmt"
	"log/slog"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/extension/richlist/evmrichlist"
	"github.com/initia-labs/rollytics/indexer/extension/richlist/moverichlist"
	richlisttypes "github.com/initia-labs/rollytics/indexer/extension/richlist/types"
	richlistutils "github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	"github.com/initia-labs/rollytics/indexer/extension/richlist/wasmrichlist"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/querier"
)

type Processor struct {
	cfg      *config.Config
	logger   *slog.Logger
	db       *orm.Database
	richlist richlisttypes.RichListProcessor
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) (*Processor, error) {
	var richlist richlisttypes.RichListProcessor
	switch cfg.GetVmType() {
	case types.MoveVM:
		richlist = moverichlist.New(cfg, logger)
	case types.EVM:
		richlist = evmrichlist.New(cfg, logger)
	case types.WasmVM:
		richlist = wasmrichlist.New(cfg)
	default:
		return nil, fmt.Errorf("rich list not supported: %v", cfg.GetVmType())
	}

	return &Processor{
		cfg:      cfg,
		logger:   logger,
		db:       db,
		richlist: richlist,
	}, nil
}

func (s *Processor) Run(ctx context.Context, startHeight int64, requireInit bool, moduleAccounts []sdk.AccAddress) error {
	currentHeight := startHeight
	q := querier.NewQuerier(s.cfg.GetChainConfig())

	if requireInit {
		s.logger.Info("reinitializing rich list", slog.Int64("start_height", currentHeight))
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			return richlistutils.InitializeBalances(ctx, q, s.logger, tx, s.cfg, currentHeight)
		}); err != nil {
			return err
		}
		currentHeight += 1
	}

	s.logger.Info("starting rich list extension", slog.Int64("start_height", currentHeight))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := s.db.Transaction(func(dbTx *gorm.DB) error {
			if _, err := richlistutils.GetCollectedBlock(ctx, dbTx, s.cfg.GetChainId(), currentHeight); err != nil {
				s.logger.Error("failed to get block", slog.Any("error", err))
				return err
			}

			cosmosTxs, err := richlistutils.GetBlockCollectedTxs(ctx, dbTx, currentHeight)
			if err != nil {
				s.logger.Error("failed to get cosmos transactions", slog.Any("error", err))
				return err
			}

			balanceMap := s.richlist.ProcessBalanceChanges(ctx, q, s.logger, cosmosTxs, moduleAccounts)
			negativeDenoms, err := richlistutils.UpdateBalanceChanges(ctx, dbTx, balanceMap)
			if err != nil {
				s.logger.Error("failed to update balance changes", slog.Any("error", err))
				return err
			}

			if err := s.richlist.AfterProcess(ctx, dbTx, currentHeight, negativeDenoms, q); err != nil {
				return err
			}

			if err := richlistutils.UpdateRichListStatus(ctx, dbTx, currentHeight); err != nil {
				s.logger.Error("failed to update rich list processed height",
					slog.Int64("height", currentHeight),
					slog.Any("error", err))
				return err
			}

			s.logger.Info("rich list processed height", slog.Int64("height", currentHeight))
			return nil
		}); err != nil {
			return err
		}

		currentHeight += 1
	}
}
