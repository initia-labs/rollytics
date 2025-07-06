package tx

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cometbft/cometbft/crypto/tmhash"
	cbjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cosmos/cosmos-sdk/codec/unknownproto"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	"gorm.io/gorm"

	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func (sub *TxSubmodule) collect(block indexertypes.ScrapedBlock, tx *gorm.DB) (err error) {
	batchSize := sub.cfg.GetDBBatchSize()
	chainId := block.ChainId
	height := block.Height

	sub.mtx.Lock()
	cacheData, ok := sub.cache[height]
	delete(sub.cache, height)
	sub.mtx.Unlock()

	if !ok {
		return errors.New("data is not prepared")
	}

	// collect fa before collecting tx (only for move)
	if err = collectFA(block, sub.cfg, tx); err != nil {
		return err
	}

	// get seq info
	seqInfo, err := indexerutil.GetSeqInfo(chainId, "tx", tx)
	if err != nil {
		return err
	}

	// create rest tx map
	restTxMap := make(map[string]RestTx) // signatures -> rest tx
	for _, restTx := range cacheData.RestTxs {
		sigKey := strings.Join(restTx.Signatures, ",")
		if sigKey != "" {
			restTxMap[sigKey] = restTx
		}
	}

	var ctxs []types.CollectedTx
	var acctxs []types.CollectedAccountTx
	accountMap := make(map[string]map[string]interface{}) // txHash -> accounts
	for txIndex, txRaw := range block.Txs {
		txByte, err := base64.StdEncoding.DecodeString(txRaw)
		if err != nil {
			return err
		}
		txHash := fmt.Sprintf("%X", tmhash.Sum(txByte))

		var raw sdktx.TxRaw
		if err = unknownproto.RejectUnknownFieldsStrict(txByte, &raw, sub.cdc.InterfaceRegistry()); err != nil {
			return err
		}

		if err = sub.cdc.Unmarshal(txByte, &raw); err != nil {
			return err
		}

		// get rest tx from map
		var sigStrings []string
		for _, sig := range raw.Signatures {
			sigStrings = append(sigStrings, base64.StdEncoding.EncodeToString(sig))
		}
		sigKey := strings.Join(sigStrings, ",")
		restTx, ok := restTxMap[sigKey]
		if !ok {
			return fmt.Errorf("no rest tx for txhash %s", txHash)
		}

		// grep msg types from rest tx
		msgTypes, err := grepMsgTypesFromRestTx(restTx)
		if err != nil {
			return err
		}

		// convert to msg type ids
		msgTypeIds, err := util.GetOrCreateMsgTypeIds(tx, msgTypes, true)
		if err != nil {
			return err
		}

		res := block.TxResults[txIndex]
		// grep type tags from events
		typeTags := grepTypeTagsFromEvents(sub.cfg, res.Events)

		// convert to type tag ids
		typeTagIds, err := util.GetOrCreateTypeTagIds(tx, typeTags, true)
		if err != nil {
			return err
		}

		var authInfo sdktx.AuthInfo
		if err = unknownproto.RejectUnknownFieldsStrict(raw.AuthInfoBytes, &authInfo, sub.cdc.InterfaceRegistry()); err != nil {
			return err
		}

		if err = sub.cdc.Unmarshal(raw.AuthInfoBytes, &authInfo); err != nil {
			return err
		}

		pubkey := authInfo.SignerInfos[0].PublicKey
		var pk cryptotypes.PubKey
		if err = sub.cdc.UnpackAny(pubkey, &pk); err != nil {
			return err
		}
		signer := sdk.AccAddress(pk.Address()).String()

		txJSON, err := cbjson.Marshal(restTx)
		if err != nil {
			return err
		}
		events, err := cbjson.Marshal(res.Events)
		if err != nil {
			return err
		}
		parsedLogs, _ := sdk.ParseABCILogs(res.Log)
		if parsedLogs == nil {
			parsedLogs = []sdk.ABCIMessageLog{}
		}
		txRes := types.Tx{
			TxHash:    txHash,
			Height:    height,
			Codespace: res.Codespace,
			Code:      res.Code,
			Data:      strings.ToUpper(hex.EncodeToString(res.Data)),
			RawLog:    res.Log,
			Logs:      parsedLogs,
			Info:      res.Info,
			GasWanted: res.GasWanted,
			GasUsed:   res.GasUsed,
			Tx:        txJSON,
			Timestamp: block.Timestamp,
			Events:    json.RawMessage(events),
		}
		txResJSON, err := cbjson.Marshal(txRes)
		if err != nil {
			return err
		}

		seqInfo.Sequence++
		ctxs = append(ctxs, types.CollectedTx{
			Hash:       txHash,
			ChainId:    chainId,
			Height:     height,
			Sequence:   seqInfo.Sequence,
			Signer:     signer,
			Data:       json.RawMessage(txResJSON),
			MsgTypeIds: msgTypeIds,
			TypeTagIds: typeTagIds,
		})

		// grep addresses for account tx
		addrs, err := grepAddressesFromTx(chainId, res.Events, tx)
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
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(ctxs, batchSize).Error; err != nil {
		return err
	}

	// insert acctxs
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(acctxs, batchSize).Error; err != nil {
		return err
	}

	// update seq info
	if err := tx.Clauses(orm.UpdateAllWhenConflict).Create(&seqInfo).Error; err != nil {
		return err
	}

	return sub.collectEvm(block, cacheData.EvmTxs, tx)
}

func (sub *TxSubmodule) collectEvm(block indexertypes.ScrapedBlock, evmTxs []types.EvmTx, tx *gorm.DB) (err error) {
	if sub.cfg.GetVmType() != types.EVM {
		return nil
	}

	batchSize := sub.cfg.GetDBBatchSize()
	chainId := block.ChainId
	height := block.Height

	// get seq info
	seqInfo, err := indexerutil.GetSeqInfo(chainId, "evm_tx", tx)
	if err != nil {
		return err
	}

	var cetxs []types.CollectedEvmTx
	var acetxs []types.CollectedEvmAccountTx
	accountMap := make(map[string]map[string]interface{})

	for _, evmTx := range evmTxs {
		txJSON, err := json.Marshal(evmTx)
		if err != nil {
			return err
		}

		signer, err := util.AccAddressFromString(evmTx.From)
		if err != nil {
			return err
		}

		seqInfo.Sequence++
		cetxs = append(cetxs, types.CollectedEvmTx{
			ChainId:  chainId,
			Hash:     evmTx.TxHash,
			Height:   height,
			Sequence: seqInfo.Sequence,
			Signer:   signer.String(),
			Data:     json.RawMessage(txJSON),
		})

		// grep addresses for account tx
		addrs, err := grepAddressesFromEvmTx(evmTx)
		if err != nil {
			return err
		}

		// initialize account list
		accountMap[evmTx.TxHash] = make(map[string]interface{})
		for _, addr := range addrs {
			accountMap[evmTx.TxHash][addr] = nil
		}
	}

	// collect account txs
	for txHash, accounts := range accountMap {
		for account := range accounts {
			acetxs = append(acetxs, types.CollectedEvmAccountTx{
				ChainId: chainId,
				Hash:    txHash,
				Account: account,
				Height:  height,
			})
		}
	}

	// insert evm txs
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(cetxs, batchSize).Error; err != nil {
		return err
	}

	// insert evm account txs
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(acetxs, batchSize).Error; err != nil {
		return err
	}

	// update seq info
	if err := tx.Clauses(orm.UpdateAllWhenConflict).Create(&seqInfo).Error; err != nil {
		return err
	}

	return nil
}
