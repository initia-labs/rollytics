package config

import "time"

type InternalTxConfig struct {
	Enabled       bool
	PollInterval  time.Duration
	BatchSize     int
}

func (c InternalTxConfig) GetPollInterval() time.Duration {
	return c.PollInterval
}
func (c InternalTxConfig) GetBatchSize() int {
	return c.BatchSize
}
