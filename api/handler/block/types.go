package block

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

type BlocksResponse struct {
	Blocks     []Block                   `json:"blocks" extensions:"x-order:0"`
	Pagination common.PaginationResponse `json:"pagination" extensions:"x-order:1"`
}

type BlockResponse struct {
	Block Block `json:"block"`
}

type AvgBlockTimeResponse struct {
	AvgBlockTime float64 `json:"avg_block_time"`
}

type Block struct {
	ChainID   string   `json:"chain_id" extensions:"x-order:0"`
	Height    string   `json:"height" extensions:"x-order:1"`
	Hash      string   `json:"hash" extensions:"x-order:2"`
	BlockTime string   `json:"block_time" extensions:"x-order:3"`
	Timestamp string   `json:"timestamp" extensions:"x-order:4"`
	GasUsed   string   `json:"gas_used" extensions:"x-order:5"`
	GasWanted string   `json:"gas_wanted" extensions:"x-order:6"`
	TxCount   string   `json:"tx_count" extensions:"x-order:7"`
	TotalFee  []Fee    `json:"total_fee" extensions:"x-order:8"`
	Proposer  Proposer `json:"proposer" extensions:"x-order:9"`
}

type Fee struct {
	Denom  string `json:"denom" extensions:"x-order:0"`
	Amount string `json:"amount" extensions:"x-order:1"`
}

type Proposer struct {
	Moniker         string `json:"moniker" extensions:"x-order:0"`
	Identity        string `json:"identity" extensions:"x-order:1"`
	OperatorAddress string `json:"operator_address" extensions:"x-order:2"`
}

func ToBlocksResponse(cbs []types.CollectedBlock, cfg *config.Config) ([]Block, error) {
	blocks := make([]Block, 0, len(cbs))
	for _, cb := range cbs {
		block, err := ToBlockResponse(cb, cfg)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func ToBlockResponse(cb types.CollectedBlock, cfg *config.Config) (block Block, err error) {
	var fees []Fee
	if err := json.Unmarshal(cb.TotalFee, &fees); err != nil {
		return block, err
	}

	validator, err := getValidator(cb.Proposer, cfg)
	if err != nil {
		return block, err
	}

	return Block{
		ChainID:   cb.ChainId,
		Height:    fmt.Sprintf("%d", cb.Height),
		Hash:      util.BytesToHexWithPrefix(cb.Hash),
		BlockTime: fmt.Sprintf("%d", cb.BlockTime),
		Timestamp: cb.Timestamp.Format(time.RFC3339),
		GasUsed:   fmt.Sprintf("%d", cb.GasUsed),
		GasWanted: fmt.Sprintf("%d", cb.GasWanted),
		TxCount:   fmt.Sprintf("%d", cb.TxCount),
		TotalFee:  fees,
		Proposer: Proposer{
			Moniker:         validator.Moniker,
			OperatorAddress: validator.OperatorAddress,
		},
	}, nil
}
