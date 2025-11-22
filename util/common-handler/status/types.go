package status

type StatusResponse struct {
	Version             string `json:"version" extensions:"x-order:0"`
	CommitHash          string `json:"commit_hash" extensions:"x-order:1"`
	ChainId             string `json:"chain_id" extensions:"x-order:2"`
	Height              int64  `json:"height" extensions:"x-order:3"`
	InternalTxHeight    int64  `json:"internal_tx_height,omitempty" extensions:"x-order:4"`
	RichListHeight      int64  `json:"rich_list_height,omitempty" extensions:"x-order:5"`
	EvmRetCleanupHeight int64  `json:"evm_ret_cleanup_height,omitempty" extensions:"x-order:6"`
}
