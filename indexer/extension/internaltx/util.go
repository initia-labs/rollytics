package internaltx

import (
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

type InternalTxInfo struct {
	Height      int64
	HashId      int64
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

	accIdMap, err := util.GetOrCreateAccountIds(db, accounts, true)
	if err != nil {
		return nil, err
	}

	var accIds []int64
	for _, acc := range accounts {
		if id, ok := accIdMap[acc]; ok {
			accIds = append(accIds, id)
		}
	}

	seqInfo.Sequence++

	// Get From and To account IDs
	var fromId, toId int64
	
	if call.From != "" {
		fromAddr, err := util.AccAddressFromString(call.From)
		if err != nil {
			return nil, err
		}
		fromId = accIdMap[fromAddr.String()]
	}
	
	if call.To != "" {
		toAddr, err := util.AccAddressFromString(call.To)
		if err != nil {
			return nil, err
		}
		toId = accIdMap[toAddr.String()]
	}
	inputBytes, err := util.HexToBytes(call.Input)
	if err != nil {
		return nil, err
	}
	outputBytes, err := util.HexToBytes(call.Output)
	if err != nil {
		return nil, err
	}
	valueBytes, err := util.HexToBytes(call.Value)
	if err != nil {
		return nil, err
	}
	gasBytes, err := util.HexToBytes(call.Gas)
	if err != nil {
		return nil, err
	}
	gasUsedBytes, err := util.HexToBytes(call.GasUsed)
	if err != nil {
		return nil, err
	}

	internalTx := types.CollectedEvmInternalTx{
		Height:      tx.Height,
		HashId:      tx.HashId,
		Sequence:    int64(seqInfo.Sequence),
		Index:       tx.Index,
		ParentIndex: tx.ParentIndex,
		Type:        call.Type,
		FromId:      fromId,
		ToId:        toId,
		Input:       inputBytes,
		Output:      outputBytes,
		Value:       valueBytes,
		Gas:         gasBytes,
		GasUsed:     gasUsedBytes,
		AccountIds:  accIds,
	}

	results := []types.CollectedEvmInternalTx{internalTx}

	// Process nested calls recursively
	nextIndex := tx.Index + 1
	for _, nestedCall := range call.Calls {
		nestedTxInfo := &InternalTxInfo{
			Height:      tx.Height,
			HashId:      tx.HashId,
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
