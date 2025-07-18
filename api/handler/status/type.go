package status

type StatusResponse struct {
	Version    string `json:"version" extensions:"x-order:0"`
	CommitHash string `json:"commit_hash" extensions:"x-order:1"`
	ChainId    string `json:"chain_id" extensions:"x-order:2"`
	Height     int64  `json:"height" extensions:"x-order:3"`
}
