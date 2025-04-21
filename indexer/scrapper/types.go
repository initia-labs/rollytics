package scrapper

import (
	abci "github.com/cometbft/cometbft/abci/types"
)

type GetBlockResponse struct {
	Result struct {
		BlockId struct {
			Hash string `json:"hash"`
		} `json:"block_id"`
		Block struct {
			Header struct {
				ChainId         string `json:"chain_id"`
				Height          string `json:"height"`
				Time            string `json:"time"`
				ProposerAddress string `json:"proposer_address"`
			} `json:"header"`
			Data struct {
				Txs []string `json:"txs"`
			} `json:"data"`
			LastCommit struct {
				Signatures []struct {
					BlockIdFlag      int32  `json:"block_id_flag"`
					ValidatorAddress string `json:"validator_address"`
				} `json:"signatures"`
			} `json:"last_commit"`
		} `json:"block"`
	} `json:"result"`
}

type GetBlockResultsResponse struct {
	Result struct {
		FinalizeBlockEvents []abci.Event        `json:"finalize_block_events"`
		TxsResults          []abci.ExecTxResult `json:"txs_results"`
	} `json:"result"`
}

type ErrorResponse struct {
	Error struct {
		Code    int64  `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	} `json:"error"`
}

type ScrapResult struct {
	Height int64
	Err    error
}
