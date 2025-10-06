package config

import "time"

type InternalTxConfig struct {
	Enabled      bool
	PollInterval time.Duration
	BatchSize    int
	QueueSize    int // Maximum number of heights in the queue

}

func (c InternalTxConfig) GetPollInterval() time.Duration {
	return c.PollInterval
}
func (c InternalTxConfig) GetBatchSize() int {
	return c.BatchSize
}
func (c InternalTxConfig) GetQueueSize() int {
	return c.QueueSize
}
