package db

import "errors"

var (
	ErrorNonRetryable   = errors.New("non-retryable error")
	ErrorLengthMismatch = errors.New("length mismatch")
)
