package tx

import (
	"encoding/json"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
)

func grepMsgTypesFromRestTx(tx RestTx) (msgTypes []string, err error) {
	msgTypeMap := make(map[string]interface{})

	var body RestTxBody
	if err := json.Unmarshal(tx.Body, &body); err != nil {
		return msgTypes, err
	}

	for _, msg := range body.Messages {
		msgType := strings.TrimPrefix(msg.Type, "/")
		msgTypeMap[msgType] = nil
	}

	for msgType := range msgTypeMap {
		msgTypes = append(msgTypes, msgType)
	}

	return
}

func grepTypeTagsFromEvents(cfg *config.Config, events []abci.Event) (typeTags []string) {
	if cfg.GetVmType() != types.MoveVM {
		return
	}

	typeTagMap := make(map[string]interface{})

	for _, event := range events {
		if event.Type != "move" {
			continue
		}

		for _, attr := range event.Attributes {
			if attr.Key == "type_tag" && attr.Value != "" {
				typeTagMap[attr.Value] = nil
			}
		}
	}

	for typeTag := range typeTagMap {
		typeTags = append(typeTags, typeTag)
	}

	return
}
