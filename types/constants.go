package types

import "time"

// Indexer constants
const (
	// Block processing thresholds
	MaxInflightBlocks = 100
	MinInflightBlocks = 50

	// Chain readiness check
	MinChainHeightToStart = 10
	ChainCheckInterval    = 5 * time.Second

	// Scraper constants
	BatchScrapSize            = 5
	MaxScrapeErrCount         = 5
	ScrapeSpeedUpdateInterval = 10 * time.Second
)
