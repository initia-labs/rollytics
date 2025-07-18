package mq

// Message is the shared message format for block metadata.
type Message struct {
	Height int64    `json:"height"`
	Hash   []string `json:"hash"`
}
