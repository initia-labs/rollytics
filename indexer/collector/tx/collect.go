package tx

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/cometbft/cometbft/crypto/tmhash"
	cbjson "github.com/cometbft/cometbft/libs/json"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func (sub TxSubmodule) collect(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	chainId := block.ChainId
	height := block.Height
	txDecode := sub.txConfig.TxDecoder()
	jsonEncoder := sub.txConfig.TxJSONEncoder()
	txSeqInfo, err := getSeqInfo(chainId, "tx", tx)
	if err != nil {
		return err
	}
	acctxSeqInfo, err := getSeqInfo(chainId, "account_tx", tx)
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

		txSeqInfo.Sequence++
		ctx := types.CollectedTx{
			Hash:     fmt.Sprintf("%X", tmhash.Sum(txByte)),
			ChainId:  chainId,
			Height:   height,
			Sequence: txSeqInfo.Sequence,
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
			acctxSeqInfo.Sequence++
			acctxs = append(acctxs, types.CollectedAccountTx{
				Hash:     txHash,
				Account:  account,
				ChainId:  chainId,
				Height:   height,
				Sequence: acctxSeqInfo.Sequence,
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
	if res := tx.Clauses(orm.UpdateAllWhenConflict).Create([]types.CollectedSeqInfo{txSeqInfo, acctxSeqInfo}); res.Error != nil {
		return res.Error
	}

	return nil
}
