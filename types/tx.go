package types

import (
	"encoding/json"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type Tx struct {
	TxHash    string              `json:"txhash"`
	Height    int64               `json:"height"`
	Codespace string              `json:"codespace"`
	Code      uint32              `json:"code"`
	Data      string              `json:"data"`
	RawLog    string              `json:"raw_log"`
	Logs      sdk.ABCIMessageLogs `json:"logs"`
	Info      string              `json:"info"`
	GasWanted int64               `json:"gas_wanted"`
	GasUsed   int64               `json:"gas_used"`
	Tx        json.RawMessage     `json:"tx"`
	Timestamp time.Time           `json:"timestamp"`
	Events    json.RawMessage     `json:"events"`
}

type EvmTx struct {
	BlockHash         string   `json:"blockHash"`
	BlockNumber       string   `json:"blockNumber"`
	ContractAddress   *string  `json:"contractAddress"`
	CumulativeGasUsed string   `json:"cumulativeGasUsed"`
	EffectiveGasPrice string   `json:"effectiveGasPrice"`
	From              string   `json:"from"`
	GasUsed           string   `json:"gasUsed"`
	Logs              []EvmLog `json:"logs"`
	LogsBloom         string   `json:"logsBloom"`
	Status            string   `json:"status"`
	To                string   `json:"to"`
	TxHash            string   `json:"transactionHash"`
	TxIndex           string   `json:"transactionIndex"`
	Type              string   `json:"type"`
}

type EvmLog struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
	BlockNumber string   `json:"blockNumber"`
	TxHash      string   `json:"transactionHash"`
	TxIndex     string   `json:"transactionIndex"`
	BlockHash   string   `json:"blockHash"`
	LogIndex    string   `json:"logIndex"`
	Removed     bool     `json:"removed"`
}

type EvmInternalTx struct {
	Type    string `json:"type"`
	From    string `json:"from"`
	To      string `json:"to"`
	Gas     string `json:"gas"`
	GasUsed string `json:"gasUsed"`
	Value   string `json:"value"`
	Input   string `json:"input"`
	Output  string `json:"output"`
}
