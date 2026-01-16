package tx

import (
	"encoding/json"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
)

func grepMsgTypesFromRestTx(tx types.RestTx) (msgTypes []string, err error) {
	msgTypeMap := make(map[string]interface{})

	var body types.RestTxBody
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

// sanitizeJSONBytes removes null bytes from JSON bytes that cannot be stored
// in PostgreSQL. PostgreSQL explicitly rejects null bytes (\u0000) in text fields.
// It replaces null bytes with the Unicode replacement character (\uFFFD) and
// handles Unicode escape sequences in JSON strings.
func sanitizeJSONBytes(data []byte) []byte {
	// Convert to string to use strings.ReplaceAll
	str := string(data)

	// Replace raw null bytes with Unicode replacement character
	str = strings.ReplaceAll(str, "\x00", "\uFFFD")

	// Replace null byte Unicode escape sequences in JSON strings
	str = strings.ReplaceAll(str, "\\u0000", "\\uFFFD")

	return []byte(str)
}
