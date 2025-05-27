package block

import (
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func (sub *BlockSubmodule) collect(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	var cb types.CollectedBlock
	cb.ChainId = block.ChainId
	cb.Height = block.Height
	cb.Hash = block.Hash
	cb.Timestamp = block.Timestamp
	if block.Height > 1 {
		prevBlock, err := getBlock(block.ChainId, block.Height-1, tx)
		if err != nil {
			return err
		}
		cb.BlockTime = block.Timestamp.Sub(prevBlock.Timestamp).Milliseconds()
	}
	cb.Proposer = block.Proposer
	cb.TotalFee, err = getTotalFee(block.Txs, sub.txConfig)
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
