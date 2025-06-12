package types

import (
	"encoding/json"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type TxByHeightRecord struct {
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
