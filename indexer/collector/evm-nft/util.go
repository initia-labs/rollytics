package evm_nft

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/cometbft/cometbft/crypto/tmhash"
	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
)

func getEvents(block indexertypes.ScrappedBlock, eventType string) (events []EventWithHash, err error) {
	for _, event := range block.BeginBlock {
		if event.Type != eventType {
			continue
		}

		events = append(events, EventWithHash{
			TxHash: "",
			Event:  event,
		})
	}

	for txIndex, txRaw := range block.Txs {
		txByte, err := base64.StdEncoding.DecodeString(txRaw)
		if err != nil {
			return events, err
		}
		txHash := fmt.Sprintf("%X", tmhash.Sum(txByte))
		txRes := block.TxResults[txIndex]
		for _, event := range txRes.Events {
			if event.Type != eventType {
				continue
			}

			events = append(events, EventWithHash{
				TxHash: txHash,
				Event:  event,
			})
		}
	}

	for _, event := range block.EndBlock {
		if event.Type != eventType {
			continue
		}

		events = append(events, EventWithHash{
			TxHash: "",
			Event:  event,
		})
	}

	return events, nil
}

func isEvmNftLog(log evmtypes.Log) bool {
	return len(log.Topics) == 4 && log.Topics[0] == nftTopic && log.Data == "0x"
}

func convertHexStringToDecString(hex string) (string, error) {
	hex = strings.TrimPrefix(hex, "0x")
	bi, ok := new(big.Int).SetString(hex, 16)
	if !ok {
		return "", errors.New("failed to convert hex to dec")
	}
	return bi.String(), nil
}
