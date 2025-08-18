package types

import (
	"fmt"
)

// Standard error types
type ErrorType string

const (
	ErrTypeConfig       ErrorType = "CONFIG_ERROR"
	ErrTypeValidation   ErrorType = "VALIDATION_ERROR"
	ErrTypeInvalidValue ErrorType = "INVALID_VALUE"
	ErrTypeDatabase     ErrorType = "DATABASE_ERROR"
	ErrTypeNetwork      ErrorType = "NETWORK_ERROR"
	ErrTypeInternal     ErrorType = "INTERNAL_ERROR"
	ErrTypeNotFound     ErrorType = "NOT_FOUND"
	ErrTypeBadRequest   ErrorType = "BAD_REQUEST"
	ErrTypeRateLimit    ErrorType = "RATE_LIMIT"
	ErrTypeTimeout      ErrorType = "TIMEOUT"
)

// StandardError provides consistent error formatting
type StandardError struct {
	Type    ErrorType
	Message string
	Details map[string]any
	Cause   error
}

func (e *StandardError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

func (e *StandardError) Unwrap() error {
	return e.Cause
}

// Error constructors for common cases

func NewConfigError(msg string, cause error) error {
	return &StandardError{
		Type:    ErrTypeConfig,
		Message: msg,
		Cause:   cause,
	}
}

func NewValidationError(field, msg string) error {
	return &StandardError{
		Type:    ErrTypeValidation,
		Message: fmt.Sprintf("validation failed for %s: %s", field, msg),
		Details: map[string]any{"field": field},
	}
}

func NewInvalidValueError(field, value, msg string) error {
	return &StandardError{
		Type:    ErrTypeInvalidValue,
		Message: fmt.Sprintf("invalid value for %s: %s (%s)", field, value, msg),
		Details: map[string]any{"field": field, "value": value},
	}
}

func NewDatabaseError(operation string, cause error) error {
	return &StandardError{
		Type:    ErrTypeDatabase,
		Message: fmt.Sprintf("database %s failed", operation),
		Cause:   cause,
	}
}

func NewNetworkError(url string, cause error) error {
	return &StandardError{
		Type:    ErrTypeNetwork,
		Message: fmt.Sprintf("network request to %s failed", url),
		Details: map[string]any{"url": url},
		Cause:   cause,
	}
}

func NewNotFoundError(resource string) error {
	return &StandardError{
		Type:    ErrTypeNotFound,
		Message: fmt.Sprintf("%s not found", resource),
		Details: map[string]any{"resource": resource},
	}
}

func NewBadRequestError(msg string) error {
	return &StandardError{
		Type:    ErrTypeBadRequest,
		Message: msg,
	}
}

func NewRateLimitError(endpoint string) error {
	return &StandardError{
		Type:    ErrTypeRateLimit,
		Message: fmt.Sprintf("rate limit exceeded for endpoint: %s", endpoint),
		Details: map[string]any{"endpoint": endpoint},
	}
}

func NewTimeoutError(operation string) error {
	return &StandardError{
		Type:    ErrTypeTimeout,
		Message: fmt.Sprintf("%s operation timed out", operation),
		Details: map[string]any{"operation": operation},
	}
}

func NewInternalError(msg string, cause error) error {
	return &StandardError{
		Type:    ErrTypeInternal,
		Message: msg,
		Cause:   cause,
	}
}

func NewInvalidHeightError() error {
	return &StandardError{
		Type:    ErrTypeBadRequest,
		Message: "invalid height: cannot query with height in the future",
	}
}

func NewLimiterNotInitializedError() error {
	return &StandardError{
		Type:    ErrTypeConfig,
		Message: "rate limiter not initialized: call InitLimiter first",
	}
}
