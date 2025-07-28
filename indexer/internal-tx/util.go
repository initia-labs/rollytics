package internal_tx

import (
	"strconv"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
	"gorm.io/gorm"
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
	accountMap map[int64]interface{},
) (*types.CollectedEvmInternalTx, error) {
	// Parse gas values
	gas := int64(0)
	if call.Gas != "" {
		var err error
		gas, err = strconv.ParseInt(call.Gas, 0, 64)
		if err != nil {
			return nil, err
		}
	}

	gasUsed := int64(0)
	if call.GasUsed != "" {
		var err error
		gasUsed, err = strconv.ParseInt(call.GasUsed, 0, 64)
		if err != nil {
			return nil, err
		}
	}

	value := int64(0)
	if call.Value != "" {
		var err error
		value, err = strconv.ParseInt(call.Value, 0, 64)
		if err != nil {
			return nil, err
		}
	}

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
	for _, accId := range accIds {
		accountMap[accId] = nil
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
		Value:      value,
		Gas:        gas,
		GasUsed:    gasUsed,
		AccountIds: accIds,
	}

	return &internalTx, nil
}
