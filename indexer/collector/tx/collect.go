package tx

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cometbft/cometbft/crypto/tmhash"
	cbjson "github.com/cometbft/cometbft/libs/json"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func (sub TxSubmodule) collect(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	chainId := block.ChainId
	height := block.Height
	txDecode := sub.txConfig.TxDecoder()
	jsonEncoder := sub.txConfig.TxJSONEncoder()
	seqInfo, err := getSeqInfo(chainId, "tx", tx)
	if err != nil {
		return err
	}

	var ctxs []types.CollectedTx
	var acctxs []types.CollectedAccountTx
	accountMap := make(map[string]map[string]interface{}) // txHash -> accounts
	for txIndex, txRaw := range block.Txs {
		txByte, err := base64.StdEncoding.DecodeString(txRaw)
		if err != nil {
			return err
		}

		decoded, decodeErr := txDecode(txByte)
		if decodeErr != nil {
			if txIndex == 0 {
				continue
			}
			return decodeErr
		}

		txJSON, _ := jsonEncoder(decoded)
		res := block.TxResults[txIndex]
		// handle response -> json

		events, err := cbjson.Marshal(res.Events)
		if err != nil {
			return err
		}

		txHash := fmt.Sprintf("%X", tmhash.Sum(txByte))
		txByHeightRecord := indexertypes.TxByHeightRecord{
			Code:      res.Code,
			Codespace: res.Codespace,
			GasUsed:   res.GasUsed,
			GasWanted: res.GasWanted,
			Height:    height,
			TxHash:    txHash,
			Timestamp: block.Timestamp,
			Tx:        txJSON,
			Events:    json.RawMessage(events),
		}
		txByHeightRecordJSON, err := cbjson.Marshal(txByHeightRecord)
		if err != nil {
			return err
		}

		seqInfo.Sequence++
		ctx := types.CollectedTx{
			Hash:     fmt.Sprintf("%X", tmhash.Sum(txByte)),
			ChainId:  chainId,
			Height:   height,
			Sequence: seqInfo.Sequence,
			Data:     json.RawMessage(txByHeightRecordJSON),
		}
		ctxs = append(ctxs, ctx)

		// grep addresses for account tx
		addrs, err := grepAddressesFromTx(res.Events)
		if err != nil {
			return err
		}

		// initialize account list
		accountMap[txHash] = make(map[string]interface{})
		for _, addr := range addrs {
			accountMap[txHash][addr] = nil
		}
	}

	// collect account txs
	for txHash, accounts := range accountMap {
		for account := range accounts {
			acctxs = append(acctxs, types.CollectedAccountTx{
				Hash:    txHash,
				Account: account,
				ChainId: chainId,
				Height:  height,
			})
		}
	}

	// insert txs
	if res := tx.Clauses(orm.DoNothingWhenConflict).Create(ctxs); res.Error != nil {
		return res.Error
	}

	// insert acctxs
	if res := tx.Clauses(orm.DoNothingWhenConflict).Create(acctxs); res.Error != nil {
		return res.Error
	}

	// update seq info
	if res := tx.Clauses(orm.UpdateAllWhenConflict).Create([]types.CollectedSeqInfo{seqInfo}); res.Error != nil {
		return res.Error
	}

	return sub.collectEvm(block, tx)
}

func (sub TxSubmodule) collectEvm(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	if sub.cfg.GetChainConfig().VmType != types.EVM {
		return nil
	}

	seqInfo, err := getSeqInfo(block.ChainId, "evm_tx", tx)
	if err != nil {
		return err
	}

	evmTxs, ok := sub.evmTxMap[block.Height]
	if !ok {
		return errors.New("data is not prepared")
	}

	var cetxs []types.CollectedEvmTx
	var acetxs []types.CollectedEvmAccountTx
	accountMap := make(map[string]map[string]interface{})

	for _, evmTx := range evmTxs {
		txJSON, err := json.Marshal(evmTx)
		if err != nil {
			return err
		}

		seqInfo.Sequence++
		cetxs = append(cetxs, types.CollectedEvmTx{
			ChainId:  block.ChainId,
			Hash:     evmTx.TxHash,
			Height:   block.Height,
			Sequence: seqInfo.Sequence,
			Data:     json.RawMessage(txJSON),
		})

		from, err := util.AccAddressFromString(evmTx.From)
		if err != nil {
			return err
		}
		to, err := util.AccAddressFromString(evmTx.To)
		if err != nil {
			return err
		}
		accountMap[evmTx.TxHash] = make(map[string]interface{})
		accountMap[evmTx.TxHash][from.String()] = nil
		accountMap[evmTx.TxHash][to.String()] = nil
	}

	for txHash, accounts := range accountMap {
		for account := range accounts {
			acetxs = append(acetxs, types.CollectedEvmAccountTx{
				ChainId: block.ChainId,
				Hash:    txHash,
				Account: account,
				Height:  block.Height,
			})
		}
	}

	// insert evm txs
	if res := tx.Clauses(orm.DoNothingWhenConflict).Create(cetxs); res.Error != nil {
		return res.Error
	}

	// insert evm account txs
	if res := tx.Clauses(orm.DoNothingWhenConflict).Create(acetxs); res.Error != nil {
		return res.Error
	}

	// update seq info
	if res := tx.Clauses(orm.UpdateAllWhenConflict).Create([]types.CollectedSeqInfo{seqInfo}); res.Error != nil {
		return res.Error
	}

	return nil
}
