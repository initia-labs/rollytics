package types

import "context"

type Extension interface {
	Name() string
	Run() error
}

// ContextAwareExtension defines extensions that support context-based cancellation
type ContextAwareExtension interface {
	Extension
	RunWithContext(ctx context.Context) error
}
