package types

import (
	"encoding/json"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"gorm.io/gorm"
)

type Submodule interface {
	Name() string
	Prepare(block ScrapedBlock) error
	Collect(block ScrapedBlock, tx *gorm.DB) error
}

type ScrapedBlock struct {
	ChainId    string
	Height     int64
	Timestamp  time.Time
	Hash       string
	Proposer   string
	Txs        []string
	TxResults  []abci.ExecTxResult
	PreBlock   []abci.Event
	BeginBlock []abci.Event
	EndBlock   []abci.Event
}

type TxResult struct {
	Code      uint32       `json:"code"`
	Codespace string       `json:"codespace"`
	Data      string       `json:"data"`
	GasWanted int64        `json:"gas_wanted"`
	GasUsed   int64        `json:"gas_used"`
	Events    []abci.Event `json:"events"`
}

type ParsedEvent struct {
	TxHash string
	abci.Event
	AttrMap map[string]string
}

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

type ErrorResponse struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}
