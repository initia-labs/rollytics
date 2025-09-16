package types

import "context"

// Extension defines the interface for all indexer extensions
// All extensions must support context-based cancellation for graceful shutdown
type Extension interface {
	Name() string
	Run(ctx context.Context) error
}
