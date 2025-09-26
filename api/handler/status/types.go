package status

type StatusResponse struct {
	Version          string `json:"version" extensions:"x-order:0"`
	CommitHash       string `json:"commit_hash" extensions:"x-order:1"`
	ChainId          string `json:"chain_id" extensions:"x-order:2"`
	Height           int64  `json:"height" extensions:"x-order:3"`
	InternalTxHeight int64  `json:"internal_tx_height,omitempty" extensions:"x-order:4"`

	// Edge backfill status
	EdgeBackfill EdgeBackfillSummary `json:"edge_backfill" extensions:"x-order:5"`
}

// EdgeBackfillSummary provides a summary of the edge backfill status for various data types.
type EdgeBackfillSummary struct {
	Tx          EdgeBackfillDetails `json:"tx"`
	EvmTx       EdgeBackfillDetails `json:"evm_tx"`
	EvmInternal EdgeBackfillDetails `json:"evm_internal"`
}

// EdgeBackfillDetails provides details about the backfill status for a specific data type.
type EdgeBackfillDetails struct {
	Completed bool  `json:"completed"`
	Sequence  int64 `json:"sequence,omitempty"`
}
