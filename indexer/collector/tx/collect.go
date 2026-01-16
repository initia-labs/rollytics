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
	"github.com/initia-labs/rollytics/util/cache"
)

func (sub *TxSubmodule) collect(block indexertypes.ScrapedBlock, tx *gorm.DB) error {
	batchSize := sub.cfg.GetDBBatchSize()
	height := block.Height

	sub.mtx.Lock()
	cacheData, ok := sub.cache[height]
	delete(sub.cache, height)
	sub.mtx.Unlock()

	if !ok {
		return errors.New("data is not prepared")
	}

	// collect fa before collecting tx (only for move)
	if err := collectFA(block, sub.cfg, tx); err != nil {
		return err
	}

	// get seq info
	seqInfo, err := indexerutil.GetSeqInfo(types.SeqInfoTx, tx)
	if err != nil {
		return err
	}

	// create rest tx map
	restTxMap := make(map[string]types.RestTx) // signatures -> rest tx
	for _, restTx := range cacheData.RestTxs {
		sigKey := strings.Join(restTx.Signatures, ",")
		if sigKey != "" {
			restTxMap[sigKey] = restTx
		}
	}

	var (
		ctxs       []types.CollectedTx
		txAccounts []types.CollectedTxAccount
		txMsgTypes []types.CollectedTxMsgType
		txTypeTags []types.CollectedTxTypeTag
	)

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
		sigStrings := make([]string, 0, len(raw.Signatures))
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
		msgTypeIdMap, err := cache.GetOrCreateMsgTypeIds(tx, msgTypes, true)
		if err != nil {
			return err
		}

		var msgTypeIds []int64
		for _, msgType := range msgTypes {
			if id, ok := msgTypeIdMap[msgType]; ok {
				msgTypeIds = append(msgTypeIds, id)
			}
		}

		res := block.TxResults[txIndex]
		// grep type tags from events
		typeTags := grepTypeTagsFromEvents(sub.cfg, res.Events)

		// convert to type tag ids
		typeTagIdMap, err := cache.GetOrCreateTypeTagIds(tx, typeTags, true)
		if err != nil {
			return err
		}

		var typeTagIds []int64
		for _, tag := range typeTags {
			if id, ok := typeTagIdMap[tag]; ok {
				typeTagIds = append(typeTagIds, id)
			}
		}

		// grep addresses from events
		addrs, err := grepAddressesFromTx(res.Events, tx)
		if err != nil {
			return err
		}

		// create account map to get unique accounts
		accountMap := make(map[string]interface{})
		for _, addr := range addrs {
			accountMap[addr] = nil
		}

		// get signer address from auth info
		var authInfo sdktx.AuthInfo
		if err = unknownproto.RejectUnknownFieldsStrict(raw.AuthInfoBytes, &authInfo, sub.cdc.InterfaceRegistry()); err != nil {
			return err
		}

		if err = sub.cdc.Unmarshal(raw.AuthInfoBytes, &authInfo); err != nil {
			return err
		}

		if len(authInfo.SignerInfos) == 0 {
			return fmt.Errorf("no signer info for txhash %s", txHash)
		}
		pubkey := authInfo.SignerInfos[0].PublicKey
		var pk cryptotypes.PubKey
		if err = sub.cdc.UnpackAny(pubkey, &pk); err != nil {
			return err
		}

		signer := sdk.AccAddress(pk.Address()).String()
		accountMap[signer] = nil

		var uniqueAccounts []string
		for account := range accountMap {
			uniqueAccounts = append(uniqueAccounts, account)
		}

		accountIdMap, err := cache.GetOrCreateAccountIds(tx, uniqueAccounts, true)
		if err != nil {
			return err
		}

		var accountIds []int64
		for _, acc := range uniqueAccounts {
			if id, ok := accountIdMap[acc]; ok {
				accountIds = append(accountIds, id)
			}
		}

		signerId := accountIdMap[signer]

		txJSON, err := cbjson.Marshal(restTx)
		if err != nil {
			return err
		}
		txJSON = sanitizeJSONBytes(txJSON)
		events, err := cbjson.Marshal(res.Events)
		if err != nil {
			return err
		}
		events = sanitizeJSONBytes(events)
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
		txResJSON = sanitizeJSONBytes(txResJSON)

		hashBytes, err := util.HexToBytes(txHash)
		if err != nil {
			return err
		}

		seqInfo.Sequence++
		currentSeq := seqInfo.Sequence
		ctxs = append(ctxs, types.CollectedTx{
			Hash:     hashBytes,
			Height:   height,
			Sequence: currentSeq,
			SignerId: signerId,
			Data:     json.RawMessage(txResJSON),
		})

		if len(accountIds) > 0 {
			accountSeen := make(map[int64]struct{}, len(accountIds))
			for _, id := range accountIds {
				if _, ok := accountSeen[id]; ok {
					continue
				}
				accountSeen[id] = struct{}{}
				txAccounts = append(txAccounts, types.CollectedTxAccount{
					AccountId: id,
					Sequence:  currentSeq,
					Signer:    id == signerId,
				})
			}
		}

		if len(msgTypeIds) > 0 {
			msgSeen := make(map[int64]struct{}, len(msgTypeIds))
			for _, id := range msgTypeIds {
				if _, ok := msgSeen[id]; ok {
					continue
				}
				msgSeen[id] = struct{}{}
				txMsgTypes = append(txMsgTypes, types.CollectedTxMsgType{
					MsgTypeId: id,
					Sequence:  currentSeq,
				})
			}
		}

		if len(typeTagIds) > 0 {
			tagSeen := make(map[int64]struct{}, len(typeTagIds))
			for _, id := range typeTagIds {
				if _, ok := tagSeen[id]; ok {
					continue
				}
				tagSeen[id] = struct{}{}
				txTypeTags = append(txTypeTags, types.CollectedTxTypeTag{
					TypeTagId: id,
					Sequence:  currentSeq,
				})
			}
		}
	}

	// insert txs
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(ctxs, batchSize).Error; err != nil {
		return err
	}

	if len(txAccounts) > 0 {
		if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(txAccounts, batchSize).Error; err != nil {
			return err
		}
	}

	if len(txMsgTypes) > 0 {
		if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(txMsgTypes, batchSize).Error; err != nil {
			return err
		}
	}

	if len(txTypeTags) > 0 {
		if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(txTypeTags, batchSize).Error; err != nil {
			return err
		}
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
	height := block.Height

	// get seq info
	seqInfo, err := indexerutil.GetSeqInfo(types.SeqInfoEvmTx, tx)
	if err != nil {
		return err
	}

	var (
		cetxs         []types.CollectedEvmTx
		evmTxAccounts []types.CollectedEvmTxAccount
	)
	for _, evmTx := range evmTxs {
		txJSON, err := json.Marshal(evmTx)
		if err != nil {
			return err
		}
		txJSON = sanitizeJSONBytes(txJSON)

		// grep addresses from events
		addrs, err := grepAddressesFromEvmTx(evmTx)
		if err != nil {
			return err
		}

		// create account map to get unique accounts
		accountMap := make(map[string]interface{})
		for _, addr := range addrs {
			accountMap[addr] = nil
		}

		signer, err := util.AccAddressFromString(evmTx.From)
		if err != nil {
			return err
		}
		signerStr := signer.String()
		accountMap[signerStr] = nil

		var uniqueAccounts []string
		for account := range accountMap {
			uniqueAccounts = append(uniqueAccounts, account)
		}

		accountIdMap, err := cache.GetOrCreateAccountIds(tx, uniqueAccounts, true)
		if err != nil {
			return err
		}
		var accountIds []int64
		for _, acc := range uniqueAccounts {
			if id, ok := accountIdMap[acc]; ok {
				accountIds = append(accountIds, id)
			}
		}

		signerId := accountIdMap[signerStr]

		hashBytes, err := util.HexToBytes(evmTx.TxHash)
		if err != nil {
			return err
		}

		seqInfo.Sequence++
		currentSeq := seqInfo.Sequence
		cetxs = append(cetxs, types.CollectedEvmTx{
			Hash:     hashBytes,
			Height:   height,
			Sequence: currentSeq,
			SignerId: signerId,
			Data:     json.RawMessage(txJSON),
		})

		if len(accountIds) > 0 {
			accountSeen := make(map[int64]struct{}, len(accountIds))
			for _, id := range accountIds {
				if _, ok := accountSeen[id]; ok {
					continue
				}
				accountSeen[id] = struct{}{}
				evmTxAccounts = append(evmTxAccounts, types.CollectedEvmTxAccount{
					AccountId: id,
					Sequence:  currentSeq,
					Signer:    id == signerId,
				})
			}
		}
	}

	// insert evm txs
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(cetxs, batchSize).Error; err != nil {
		return err
	}

	if len(evmTxAccounts) > 0 {
		if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(evmTxAccounts, batchSize).Error; err != nil {
			return err
		}
	}

	// update seq info
	if err := tx.Clauses(orm.UpdateAllWhenConflict).Create(&seqInfo).Error; err != nil {
		return err
	}

	return nil
}
