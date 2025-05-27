package tx

import codectypes "github.com/cosmos/cosmos-sdk/codec/types"

type QueryEvmTxsResponse struct {
	Result []EvmTx `json:"result"`
}

type EvmTx struct {
	BlockHash         string   `json:"blockHash"`
	BlockNumber       string   `json:"blockNumber"`
	ContractAddress   *string  `json:"contractAddress"`
	CumulativeGasUsed string   `json:"cumulativeGasUsed"`
	EffectiveGasPrice string   `json:"effectiveGasPrice"`
	From              string   `json:"from"`
	GasUsed           string   `json:"gasUsed"`
	Logs              []EvmLog `json:"logs"`
	LogsBloom         string   `json:"logsBloom"`
	Status            string   `json:"status"`
	To                string   `json:"to"`
	TxHash            string   `json:"transactionHash"`
	TxIndex           string   `json:"transactionIndex"`
	Type              string   `json:"type"`
}

type EvmLog struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
	BlockNumber string   `json:"blockNumber"`
	TxHash      string   `json:"transactionHash"`
	TxIndex     string   `json:"transactionIndex"`
	BlockHash   string   `json:"blockHash"`
	LogIndex    string   `json:"logIndex"`
	Removed     bool     `json:"removed"`
}

type intoAny interface {
	AsAny() *codectypes.Any
}
