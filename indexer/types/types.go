package types

import (
	"context"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"gorm.io/gorm"
)

type Submodule interface {
	Name() string
	Prepare(ctx context.Context, block ScrapedBlock) error
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

	// RawTxs, when non-nil, holds raw tx bytes for each tx (e.g. when block was recovered from DA layer).
	// The tx prepare step will build RestTx from these instead of calling the REST API.
	RawTxs [][]byte
}

type ParsedEvent struct {
	TxHash string
	abci.Event
	AttrMap map[string]string
}

type NftCount struct {
	CollectionAddr []byte
	Count          int64
}
