package block

import (
	"errors"
	"log/slog"

	"gorm.io/gorm"

	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func (sub *BlockSubmodule) collect(block indexertypes.ScrapedBlock, tx *gorm.DB) (err error) {
	hashBytes, err := util.HexToBytes(block.Hash)
	if err != nil {
		return err
	}

	var cb types.CollectedBlock
	cb.ChainId = block.ChainId
	cb.Height = block.Height
	cb.Hash = hashBytes
	cb.Timestamp = block.Timestamp
	if block.Height > 1 {
		prevHeight := block.Height - 1

		// If a start height is configured and the previous height is below it,
		// we are starting mid-chain. Skip BlockTime computation and log it.
		if sub.cfg != nil && sub.cfg.StartHeightSet() && prevHeight < sub.cfg.GetStartHeight() {
			sub.logger.Info("starting mid-chain: skipping BlockTime for first processed block",
				slog.Int64("height", block.Height),
				slog.Int64("prev_height", prevHeight),
				slog.Int64("configured_start_height", sub.cfg.GetStartHeight()),
			)
		} else {
			prevBlock, err := GetBlock(block.ChainId, prevHeight, tx)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// starting mid-chain: previous block is not in DB yet; skip BlockTime for the first processed block
					sub.logger.Info("starting mid-chain: previous block not found; skipping BlockTime",
						slog.Int64("height", block.Height),
						slog.Int64("prev_height", prevHeight),
					)
				} else {
					return err
				}
			} else {
				cb.BlockTime = block.Timestamp.Sub(prevBlock.Timestamp).Milliseconds()
			}
		}
	}
	cb.Proposer = block.Proposer
	cb.TotalFee, err = getTotalFee(block.Txs, sub.cdc)
	if err != nil {
		return err
	}
	cb.TxCount = len(block.Txs)
	cb.GasUsed = 0
	cb.GasWanted = 0
	for txIndex := range block.Txs {
		res := block.TxResults[txIndex]
		cb.GasUsed += res.GasUsed
		cb.GasWanted += res.GasWanted
	}

	return tx.Clauses(orm.DoNothingWhenConflict).Create(&cb).Error
}
