package types

import (
	"encoding/json"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"gorm.io/gorm"
)

type Submodule interface {
	Name() string
	Prepare(block ScrappedBlock) error
	Collect(block ScrappedBlock, tx *gorm.DB) error
}

type ScrappedBlock struct {
	ChainId    string
	Height     int64
	Timestamp  time.Time
	Hash       string
	Proposer   string
	Txs        []string
	TxResults  []abci.ExecTxResult
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
	TxHash     string
	Type       string
	Attributes map[string]string
}

type TxByHeightRecord struct {
	Code      uint32          `json:"code"`
	Codespace string          `json:"codespace"`
	GasUsed   int64           `json:"gas_used"`
	GasWanted int64           `json:"gas_wanted"`
	Height    int64           `json:"height"`
	TxHash    string          `json:"txhash"`
	Timestamp time.Time       `json:"timestamp"`
	Tx        json.RawMessage `json:"tx"`
	Events    json.RawMessage `json:"events"`
}

type ErrorResponse struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}
