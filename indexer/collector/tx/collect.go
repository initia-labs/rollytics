package tx

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cometbft/cometbft/crypto/tmhash"
	cbjson "github.com/cometbft/cometbft/libs/json"
	movetypes "github.com/initia-labs/initia/x/move/types"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func (sub TxSubmodule) collectTx(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	chainId := block.ChainId
	seqInfo, err := getSeqInfo(chainId, "tx", tx)
	if err != nil {
		return err
	}

	txDecode := sub.txConfig.TxDecoder()
	jsonEncoder := sub.txConfig.TxJSONEncoder()
	var ctxs []types.CollectedTx

	for txIndex, txRaw := range block.Txs {
		txByte, err := base64.StdEncoding.DecodeString(txRaw)
		if err != nil {
			return err
		}

		txByHeightRecord := indexertypes.TxByHeightRecord{}
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

		txByHeightRecord.Code = res.Code
		txByHeightRecord.Codespace = res.Codespace
		txByHeightRecord.GasUsed = res.GasUsed
		txByHeightRecord.GasWanted = res.GasWanted
		txByHeightRecord.Height = block.Height
		txByHeightRecord.TxHash = fmt.Sprintf("%X", tmhash.Sum(txByte))
		txByHeightRecord.Timestamp = block.Timestamp
		txByHeightRecord.Tx = txJSON
		txByHeightRecord.Events = json.RawMessage(events)

		txByHeightRecordJSON, txByHeightRecordErr := cbjson.Marshal(txByHeightRecord)
		if txByHeightRecordErr != nil {
			return txByHeightRecordErr
		}

		seqInfo.Sequence++
		ctx := types.CollectedTx{
			Hash:     fmt.Sprintf("%X", tmhash.Sum(txByte)),
			ChainId:  chainId,
			Height:   block.Height,
			Sequence: seqInfo.Sequence,
			Data:     json.RawMessage(txByHeightRecordJSON),
		}
		ctxs = append(ctxs, ctx)
	}

	// insert txs
	if res := tx.Clauses(orm.DoNothingWhenConflict).Create(ctxs); res.Error != nil {
		return res.Error
	}

	// update seq info
	if res := tx.Clauses(orm.UpdateAllWhenConflict).Create(&seqInfo); res.Error != nil {
		return res.Error
	}

	return nil
}

func (sub TxSubmodule) collectAccountTx(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	chainId := block.ChainId
	height := block.Height

	seqInfo, err := getSeqInfo(chainId, "account_tx", tx)
	if err != nil {
		return err
	}

	var acctxs []types.CollectedAccountTx
	accountMap := make(map[string]map[string]interface{}) // txHash -> accounts
	for txIndex, txRaw := range block.Txs {
		txByte, err := base64.StdEncoding.DecodeString(txRaw)
		if err != nil {
			return err
		}
		txHash := fmt.Sprintf("%X", tmhash.Sum(txByte))

		for _, evt := range block.TxResults[txIndex].Events {
			parsed := parseEvent(evt)
			attrs := parsed.Attributes

			// initialize account list
			if _, ok := accountMap[txHash]; !ok {
				accountMap[txHash] = make(map[string]interface{})
			}

			// extract accounts from guid
			if evt.Type == movetypes.EventTypeMove {
				// handle hex addresses
				key, keyOk := attrs["key"]
				if !keyOk {
					continue
				}

				guid := new(struct {
					Id struct {
						Addr        string `json:"addr"`
						CreationNum string `json:"creation_num"`
					} `json:"id"`
				})
				if err := json.Unmarshal([]byte(key), guid); err != nil {
					return err
				}

				addr, err := accAddressFromString(guid.Id.Addr)
				if err != nil {
					return err
				}
				accountMap[txHash][addr.String()] = nil
			}

			// extract accounts from attributes
			for _, attrValue := range attrs {
				if strings.HasPrefix(attrValue, "init1") {
					for _, addr := range findAllBech32Address(attrValue) {
						accountMap[txHash][addr] = nil
					}
				} else if strings.HasPrefix(attrValue, "0x") {
					for _, addr := range findAllHexAddress(attrValue) {
						bechAddr, _ := accAddressFromString(addr)
						accountMap[txHash][bechAddr.String()] = nil
					}
				}
			}
		}
	}

	for txHash, accounts := range accountMap {
		for account := range accounts {
			seqInfo.Sequence++
			acctxs = append(acctxs, types.CollectedAccountTx{
				Hash:     txHash,
				Account:  account,
				ChainId:  chainId,
				Height:   height,
				Sequence: seqInfo.Sequence,
			})
		}
	}

	// insert txs
	if res := tx.Clauses(orm.DoNothingWhenConflict).Create(acctxs); res.Error != nil {
		return res.Error
	}

	// update seq info
	if res := tx.Clauses(orm.UpdateAllWhenConflict).Create(&seqInfo); res.Error != nil {
		return res.Error
	}

	return nil
}
