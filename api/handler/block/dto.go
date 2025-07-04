package block

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
)

// Request
type BlocksRequest struct {
	Pagination *common.PaginationParams `query:"pagination"`
}

type BlockByHeightRequest struct {
	Height int64 `param:"height"`
}

type AvgBlockTimeRequest struct{}

// Response
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

// Response types for blocks
type BlocksResponse struct {
	Blocks     []Block             `json:"blocks" extensions:"x-order:0"`
	Pagination common.PageResponse `json:"pagination" extensions:"x-order:1"`
}

type BlockResponse struct {
	Block *Block `json:"block"`
}

type AvgBlockTimeResponse struct {
	AvgBlockTime float64 `json:"avg_block_time"`
}

func ToResponseBlock(cb *types.CollectedBlock, restUrl string) (*Block, error) {
	var fees []Fee
	if err := json.Unmarshal(cb.TotalFee, &fees); err != nil {
		return nil, err
	}

	validator, err := getValidator(restUrl, cb.Proposer)
	if err != nil {
		return nil, err
	}
	return &Block{
		ChainID:   cb.ChainId,
		Height:    strconv.FormatInt(cb.Height, 10),
		Hash:      cb.Hash,
		BlockTime: strconv.FormatInt(cb.BlockTime, 10),
		Timestamp: cb.Timestamp.Format(time.RFC3339),
		GasUsed:   strconv.FormatInt(cb.GasUsed, 10),
		GasWanted: strconv.FormatInt(cb.GasWanted, 10),
		TxCount:   strconv.Itoa(cb.TxCount),
		TotalFee:  fees,
		Proposer: Proposer{
			Moniker:         validator.Validator.Moniker,
			OperatorAddress: validator.Validator.OperatorAddress,
		},
	}, nil
}

func BatchToResponseBlocks(cbs []types.CollectedBlock, restUrl string) ([]Block, error) {
	blocks := make([]Block, 0, len(cbs))
	for _, cb := range cbs {
		block, err := ToResponseBlock(&cb, restUrl)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, *block)
	}
	return blocks, nil
}
