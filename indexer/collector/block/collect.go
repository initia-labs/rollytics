package block

import (
	"errors"

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
		prevBlock, err := GetBlock(block.ChainId, block.Height-1, tx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// starting mid-chain: previous block is not in DB yet; skip BlockTime for the first processed block
			} else {
				return err
			}
		} else {
			cb.BlockTime = block.Timestamp.Sub(prevBlock.Timestamp).Milliseconds()
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
