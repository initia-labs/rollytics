package types

type ValidatorResponse struct {
	Validator Validator `json:"validator"`
}

type Validator struct {
	Moniker         string          `json:"moniker"`
	OperatorAddress string          `json:"operator_address"`
	ConsensusPubkey ConsensusPubkey `json:"consensus_pubkey"`
}

type ConsensusPubkey struct {
	Type string `json:"@type"`
	Key  string `json:"key"`
}
