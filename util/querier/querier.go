package querier

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/sentry_integration"
	"github.com/initia-labs/rollytics/types"
)

const (
	queryTimeout     = 5 * time.Second
	maxRetriesPerURL = 5
)

type Querier struct {
	ChainId              string
	VmType               types.VMType
	RpcUrls              []string
	RestUrls             []string
	JsonRpcUrls          []string
	AccountAddressPrefix string
	Environment          string
}

// QueryCallResponse represents the response from EVM call endpoint
type QueryCallResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

func extractResponse[T any](response []byte) (T, error) {
	var t T
	if err := json.Unmarshal(response, &t); err != nil {
		return t, err
	}
	return t, nil
}

// requestFunc is a function type that performs an HTTP request with a given endpoint URL
type requestFunc[T any] func(ctx context.Context, endpointURL string) (*T, error)

func NewQuerier(cfg *config.ChainConfig) *Querier {
	return &Querier{
		ChainId:              cfg.ChainId,
		VmType:               cfg.VmType,
		RpcUrls:              cfg.RpcUrls,
		RestUrls:             cfg.RestUrls,
		JsonRpcUrls:          cfg.JsonRpcUrls,
		AccountAddressPrefix: cfg.AccountAddressPrefix,
		Environment:          cfg.Environment,
	}
}

// executeWithEndpointRotation executes a request function with endpoint rotation and backoff.
// It rotates through the provided endpoints when maxRetriesPerURL is exceeded for the current endpoint.
func executeWithEndpointRotation[T any](ctx context.Context, endpoints []string, requestFn requestFunc[T]) (*T, error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints configured")
	}

	// Track retries per endpoint
	retriesPerEndpoint := 0
	currentEndpointIndex := 0
	totalRetries := 0
	loopSize := len(endpoints) * maxRetriesPerURL
	var lastErr error
	startEndpointIndex := 0

	for {
		// Check if context is cancelled before proceeding
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Rotate endpoint if we've exceeded maxRetriesPerURL for current endpoint
		if retriesPerEndpoint >= maxRetriesPerURL {
			// Perform backoff before rotating to next endpoint
			backoffDelay := calculateBackoffDelay(retriesPerEndpoint)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDelay):
			}

			// Rotate to next endpoint
			currentEndpointIndex = (currentEndpointIndex + 1) % len(endpoints)
			retriesPerEndpoint = 0

			// If we've looped through all endpoints, we've exhausted all options
			if currentEndpointIndex == startEndpointIndex {
				return nil, fmt.Errorf("exhausted all endpoints: %w", lastErr)
			}
		}

		// Execute the request with current endpoint
		res, err := requestFn(ctx, endpoints[currentEndpointIndex])
		if err == nil {
			return res, nil
		}

		// Request failed (including timeout), increment retry counters
		lastErr = err
		retriesPerEndpoint++
		totalRetries++

		if totalRetries == loopSize {
			sentry_integration.CaptureCurrentHubException(lastErr, sentry.LevelError)
			// If we've exhausted all retries, return the last error
			return nil, fmt.Errorf("exhausted all retries: %w", lastErr)
		}

		// Perform backoff before retrying
		backoffDelay := calculateBackoffDelay(retriesPerEndpoint)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoffDelay):
		}
	}
}
