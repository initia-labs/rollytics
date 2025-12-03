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
