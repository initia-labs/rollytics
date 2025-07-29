package internaltx

// DebugCallTraceBlockResponse represents the response from debug_traceBlockByNumber RPC call
type DebugCallTraceBlockResponse struct {
	Result []TransactionTrace `json:"result"`
}

type TransactionTrace struct {
	TxHash string `json:"txHash"`
	Result struct {
		Type    string                `json:"type"`
		From    string                `json:"from"`
		To      string                `json:"to"`
		Value   string                `json:"value"`
		Gas     string                `json:"gas"`
		GasUsed string                `json:"gasUsed"`
		Input   string                `json:"input"`
		Calls   []InternalTransaction `json:"calls,omitempty"`
	} `json:"result"`
}

type InternalTransaction struct {
	Type    string `json:"type"`
	From    string `json:"from"`
	To      string `json:"to"`
	Value   string `json:"value"`
	Gas     string `json:"gas"`
	GasUsed string `json:"gasUsed"`
	Input   string `json:"input"`
	Output  string `json:"output"`
}
