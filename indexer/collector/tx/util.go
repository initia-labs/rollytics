package tx

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func getSeqInfo(chainId string, name string, tx *gorm.DB) (seqInfo types.CollectedSeqInfo, err error) {
	if res := tx.Where("chain_id = ? AND name = ?", chainId, name).Take(&seqInfo); res.Error != nil {
		// initialize if not found
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			seqInfo = types.CollectedSeqInfo{
				ChainId:  chainId,
				Name:     name,
				Sequence: 0,
			}
		} else {
			return seqInfo, res.Error
		}
	}

	return seqInfo, nil
}

func grepMsgTypesFromRestTx(tx RestTx) (msgTypes []string, err error) {
	msgTypeMap := make(map[string]interface{})

	var body RestTxBody
	if err := json.Unmarshal(tx.Body, &body); err != nil {
		return msgTypes, err
	}

	for _, msg := range body.Messages {
		msgType := msg.Type
		if strings.HasPrefix(msgType, "/") {
			msgType = msgType[1:]
		}
		msgTypeMap[msgType] = nil
	}

	for msgType := range msgTypeMap {
		msgTypes = append(msgTypes, msgType)
	}

	return
}
