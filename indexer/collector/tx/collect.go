package tx

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cometbft/cometbft/crypto/tmhash"
	cbjson "github.com/cometbft/cometbft/libs/json"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func (sub *TxSubmodule) collect(block indexertypes.ScrapedBlock, tx *gorm.DB) (err error) {
	// collect fa before collecting tx (only for move)
	if err = collectFA(block, sub.cfg, tx); err != nil {
		return err
	}

	batchSize := sub.cfg.GetDBBatchSize()
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
			return decodeErr
		}

		p, ok := decoded.(intoAny)
		if !ok {
			return fmt.Errorf("expecting a type implementing intoAny, got :%+v", decoded)
		}
		asAny := p.AsAny()
		cachedVal := asAny.GetCachedValue()
		if cachedVal == nil {
			return errors.New("cached transaction is nil")
		}

		protoTx, ok := cachedVal.(*txtypes.Tx)
		if !ok {
			return fmt.Errorf("failed to convert to proto transaction: unexpected type %+v", cachedVal)
		}

		authInfo := protoTx.GetAuthInfo()
		pubkey := authInfo.SignerInfos[0].PublicKey

		var pk cryptotypes.PubKey
		if err = sub.cdc.UnpackAny(pubkey, &pk); err != nil {
			return err
		}
		signer := sdk.AccAddress(pk.Address()).String()

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
		ctxs = append(ctxs, types.CollectedTx{
			Hash:     fmt.Sprintf("%X", tmhash.Sum(txByte)),
			ChainId:  chainId,
			Height:   height,
			Sequence: seqInfo.Sequence,
			Signer:   signer,
			Data:     json.RawMessage(txByHeightRecordJSON),
		})

		// grep addresses for account tx
		addrs, err := grepAddressesFromTx(block.ChainId, res.Events, tx)
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
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(ctxs, batchSize); res.Error != nil {
		return res.Error
	}

	// insert acctxs
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(acctxs, batchSize); res.Error != nil {
		return res.Error
	}

	// update seq info
	if res := tx.Clauses(orm.UpdateAllWhenConflict).Create(&seqInfo); res.Error != nil {
		return res.Error
	}

	return sub.collectEvm(block, tx)
}

func (sub *TxSubmodule) collectEvm(block indexertypes.ScrapedBlock, tx *gorm.DB) (err error) {
	if sub.cfg.GetVmType() != types.EVM {
		return nil
	}

	batchSize := sub.cfg.GetDBBatchSize()
	seqInfo, err := getSeqInfo(block.ChainId, "evm_tx", tx)
	if err != nil {
		return err
	}

	sub.mtx.Lock()
	evmTxs, ok := sub.evmTxMap[block.Height]
	sub.mtx.Unlock()
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
				ChainId: block.ChainId,
				Hash:    txHash,
				Account: account,
				Height:  block.Height,
			})
		}
	}

	// insert evm txs
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(cetxs, batchSize); res.Error != nil {
		return res.Error
	}

	// insert evm account txs
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(acetxs, batchSize); res.Error != nil {
		return res.Error
	}

	// update seq info
	if res := tx.Clauses(orm.UpdateAllWhenConflict).Create(&seqInfo); res.Error != nil {
		return res.Error
	}

	sub.mtx.Lock()
	delete(sub.evmTxMap, block.Height)
	sub.mtx.Unlock()

	return nil
}
