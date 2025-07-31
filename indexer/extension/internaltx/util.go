package internaltx

import (
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

type InternalTxInfo struct {
	Height      int64
	Hash        string
	Index       int64
	ParentIndex int64
}

func processInternalCall(
	db *gorm.DB,
	tx *InternalTxInfo,
	call *InternalTransaction,
	seqInfo *types.CollectedSeqInfo,
) ([]types.CollectedEvmInternalTx, error) {
	evmInternalTx := EvmInternalTx{
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

	accIds, err := util.GetOrCreateAccountIds(db, accounts, true)
	if err != nil {
		return nil, err
	}

	seqInfo.Sequence++

	internalTx := types.CollectedEvmInternalTx{
		Height:      tx.Height,
		Hash:        tx.Hash,
		Sequence:    int64(seqInfo.Sequence),
		Index:       tx.Index,
		ParentIndex: tx.ParentIndex,
		Type:        call.Type,
		From:        call.From,
		To:          call.To,
		Input:       call.Input,
		Output:      call.Output,
		Value:       call.Value,
		Gas:         call.Gas,
		GasUsed:     call.GasUsed,
		AccountIds:  accIds,
	}

	results := []types.CollectedEvmInternalTx{internalTx}

	// Process nested calls recursively
	nextIndex := tx.Index + 1
	for _, nestedCall := range call.Calls {
		nestedTxInfo := &InternalTxInfo{
			Height:      tx.Height,
			Hash:        tx.Hash,
			Index:       nextIndex,
			ParentIndex: tx.Index,
		}
		nestedResults, err := processInternalCall(db, nestedTxInfo, &nestedCall, seqInfo)
		if err != nil {
			return nil, err
		}
		results = append(results, nestedResults...)
		nextIndex += int64(len(nestedResults))
	}

	return results, nil
}
