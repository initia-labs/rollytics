package internaltx

import (
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

type InternalTxInfo struct {
	Height int64
	Hash   string
	Index  int64
}

func processInternalCall(
	db *gorm.DB,
	tx *InternalTxInfo,
	call *InternalTransaction,
	seqInfo *types.CollectedSeqInfo,
) (*types.CollectedEvmInternalTx, error) {
	evmInternalTx := types.EvmInternalTx{
		Type:    call.Type,
		From:    call.From,
		To:      call.To,
		Gas:     call.Gas,
		GasUsed: call.GasUsed,
		Value:   call.Value,
		Input:   call.Input,
		Output:  call.Output,
	}
	accounts, err := GrepAddressesFromEvmInternalTx(evmInternalTx)
	if err != nil {
		return nil, err
	}

	// Get account IDs
	accIds, err := util.GetOrCreateAccountIds(db, accounts, true)
	if err != nil {
		return nil, err
	}

	seqInfo.Sequence++

	// Create internal tx record
	internalTx := types.CollectedEvmInternalTx{
		Height:     tx.Height,
		Hash:       tx.Hash,
		Sequence:   int64(seqInfo.Sequence),
		Index:      tx.Index,
		Type:       call.Type,
		From:       call.From,
		To:         call.To,
		Input:      call.Input,
		Output:     call.Output,
		Value:      call.Value,
		Gas:        call.Gas,
		GasUsed:    call.GasUsed,
		AccountIds: accIds,
	}

	return &internalTx, nil
}
