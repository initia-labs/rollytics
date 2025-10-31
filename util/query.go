package util

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/semaphore"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/metrics"
	"github.com/initia-labs/rollytics/types"
)

const (
	maxRetries        = 5
	baseBackoffDelay  = 1 * time.Second
	maxBackoffDelay   = 30 * time.Second
	backoffMultiplier = 2.0
	jitterFactor      = 0.1
	defaultTimeout    = 30 * time.Second
)

var (
	limiter             *semaphore.Weighted
	jitterSeed          atomic.Uint32
	cfg                 *config.Config
	endpointCategorizer *EndpointCategorizer
)

func init() {
	jitterSeed.Store(uint32(time.Now().UnixNano())) //nolint:gosec
}

// EndpointCategorizer categorizes API endpoints based on VM type
type EndpointCategorizer struct {
	vmType types.VMType
}

// NewEndpointCategorizer creates a new endpoint categorizer for the given VM type
func NewEndpointCategorizer(vmType types.VMType) *EndpointCategorizer {
	return &EndpointCategorizer{
		vmType: vmType,
	}
}

// Categorize returns the category for a given API path
func (ec *EndpointCategorizer) Categorize(path string) string {
	switch {
	// Common categories for all VMs
	case strings.HasPrefix(path, "/cosmos/tx/v1beta1/txs/block/"):
		return "cosmos_tx_block"
	case strings.HasPrefix(path, "/cosmos/tx/v1beta1/tx/"):
		return "cosmos_tx_hash"
	case strings.HasPrefix(path, "/cosmos/bank/v1beta1/balances/"):
		return "cosmos_balance"
	case strings.HasPrefix(path, "/cosmos/auth/v1beta1/accounts/"):
		return "cosmos_account"
	case path == "/block" || strings.HasPrefix(path, "/block?"):
		return "tendermint_block"
	case path == "/block_results":
		return "tendermint_block_results"
	case strings.HasPrefix(path, "/cosmos/base/tendermint/"):
		return "cosmos_node_info"
	case strings.HasPrefix(path, "/cosmos/staking/"):
		return "cosmos_staking"
	case strings.HasPrefix(path, "/cosmos/tx/v1beta1/txs"):
		return "cosmos_tx_search"

	// VM-specific categories
	case strings.HasPrefix(path, "/cosmwasm/wasm/v1/contract/"):
		if ec.vmType == types.WasmVM {
			return "wasm_contract" // Wasm only
		}
		return "other"

	case strings.HasPrefix(path, "/initia/move/v1/"):
		if ec.vmType == types.MoveVM {
			return "move_contract" // Move only
		}
		return "other"

	case path == "": // JSON-RPC endpoints have empty path
		if ec.vmType == types.EVM {
			return "evm_jsonrpc" // EVM only
		}
		return "other"

	default:
		return "other"
	}
}

func InitUtil(_cfg *config.Config) {
	if _cfg == nil {
		panic("InitUtil: config cannot be nil")
	}
	cfg = _cfg
	limiter = semaphore.NewWeighted(int64(_cfg.GetMaxConcurrentRequests()))
	endpointCategorizer = NewEndpointCategorizer(_cfg.GetVmType())
}

func acquireLimiter(ctx context.Context) error {
	if limiter == nil {
		return types.NewLimiterNotInitializedError()
	}
	return limiter.Acquire(ctx, 1)
}

func releaseLimiter() {
	if limiter != nil {
		limiter.Release(1)
	}
}

// simpleRandom generates pseudo-random number using Linear Congruential Generator
// without external dependencies, thread-safe using atomic operations
func simpleRandom() float64 {
	for {
		oldSeed := jitterSeed.Load()
		// LCG with standard constants (used by glibc)
		newSeed := oldSeed*1103515245 + 12345
		if jitterSeed.CompareAndSwap(oldSeed, newSeed) {
			// Return value between 0.0 and 1.0
			return float64(newSeed&0x7FFFFFFF) / float64(0x7FFFFFFF)
		}
		// If CAS failed, retry with new value
	}
}

// calculateBackoffDelay calculates exponential backoff delay with jitter
func calculateBackoffDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return baseBackoffDelay
	}

	// Work with seconds as float64 to avoid precision issues
	baseSeconds := baseBackoffDelay.Seconds()
	maxSeconds := maxBackoffDelay.Seconds()

	delaySeconds := baseSeconds * math.Pow(backoffMultiplier, float64(attempt-1))

	// Cap the delay at maxBackoffDelay
	if delaySeconds > maxSeconds {
		delaySeconds = maxSeconds
	}

	// Add jitter to avoid thundering herd using LCG
	jitter := delaySeconds * jitterFactor * (2*simpleRandom() - 1) // +/- jitterFactor
	delaySeconds += jitter

	// Ensure minimum delay
	if delaySeconds < baseSeconds {
		delaySeconds = baseSeconds
	}

	// Convert back to Duration safely
	// Use millisecond precision to avoid floating point issues
	durationMs := int64(delaySeconds*1000 + 0.5) // +0.5 for proper rounding
	return time.Duration(durationMs) * time.Millisecond
}

type ErrorResponse struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

type requestConfig struct {
	method  string
	params  map[string]string
	payload any
	headers map[string]string
}

func Get(ctx context.Context, baseUrl, path string, params map[string]string, headers map[string]string) ([]byte, error) {
	config := requestConfig{
		method:  fiber.MethodGet,
		params:  params,
		headers: headers,
	}
	return executeWithRetry(ctx, baseUrl, path, config)
}

func Post(ctx context.Context, baseUrl, path string, payload any, headers map[string]string) ([]byte, error) {
	config := requestConfig{
		method:  fiber.MethodPost,
		payload: payload,
		headers: headers,
	}
	return executeWithRetry(ctx, baseUrl, path, config)
}

func executeWithRetry(ctx context.Context, baseUrl, path string, config requestConfig) ([]byte, error) {
	retryCount := 0
	rateLimitRetries := 0
	// Initialize with a default error to ensure err is never nil after retries
	var err = types.NewTimeoutError("no attempts made")

	for retryCount < maxRetries {
		body, innerErr := executeHTTPRequest(ctx, baseUrl, path, config)
		if innerErr == nil {
			return body, nil
		}
		err = innerErr

		// handle case of querying future height using error type
		var standardErr *types.StandardError
		if errors.As(innerErr, &standardErr) && standardErr.Type == types.ErrTypeBadRequest &&
			strings.Contains(standardErr.Message, "invalid height") {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(cfg.GetCoolingDuration()):
				continue
			}
		}

		// handle 429 Too Many Requests with exponential backoff
		// This doesn't count against regular retry limit to allow for rate limit recovery
		if errors.Is(innerErr, fiber.ErrTooManyRequests) {
			rateLimitRetries++

			// Track rate limit hits using batching
			endpointCategory := endpointCategorizer.Categorize(path)
			metrics.GetGlobalMetricsBatcher().RecordRateLimitHit(endpointCategory)

			backoffDelay := calculateBackoffDelay(rateLimitRetries)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDelay):
				continue
			}
		}

		retryCount++
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(cfg.GetCoolingDuration()):
		}
	}

	return nil, err
}

func executeHTTPRequest(ctx context.Context, baseUrl, path string, config requestConfig) (body []byte, err error) {
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	// Validate URL for security and get parsed URL (prevents duplicate parsing)
	parsedUrl, err := validateAndParseURL(baseUrl, path)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	// Categorize endpoint to reduce metric cardinality
	endpointCategory := endpointCategorizer.Categorize(path)

	// Track concurrent requests using batching
	metrics.GetGlobalMetricsBatcher().RecordConcurrentRequest(1)
	defer func() {
		metrics.GetGlobalMetricsBatcher().RecordConcurrentRequest(-1)
		// Record API latency using endpoint category
		duration := time.Since(start).Seconds()
		metrics.GetGlobalMetricsBatcher().RecordAPILatency(endpointCategory, duration)
	}()

	// Track semaphore wait time
	semaphoreStart := time.Now()
	if err := acquireLimiter(ctx); err != nil {
		return nil, types.NewInternalError("failed to acquire semaphore", err)
	}
	defer releaseLimiter()

	// Record semaphore wait duration using batching
	semaphoreWaitTime := time.Since(semaphoreStart).Seconds()
	metrics.GetGlobalMetricsBatcher().RecordSemaphoreWait(semaphoreWaitTime)

	var req *fiber.Agent
	if config.method == fiber.MethodGet {
		// set query params
		if config.params != nil {
			query := parsedUrl.Query()
			for key, value := range config.params {
				query.Set(key, value)
			}
			parsedUrl.RawQuery = query.Encode()
		}

		req = client.Get(parsedUrl.String())
	} else {
		req = client.Post(parsedUrl.String())

		// set payload for POST
		if config.payload != nil {
			req = req.JSON(config.payload)
		}
	}

	// set headers
	for key, value := range config.headers {
		req.Set(key, value)
	}

	// Extract timeout from context if available
	timeout := defaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	code, body, errs := req.Timeout(timeout).Bytes()
	if err := errors.Join(errs...); err != nil {
		// Track network/timeout errors using batching
		metrics.GetGlobalMetricsBatcher().RecordAPIRequest(endpointCategory, "error")
		return nil, types.NewNetworkError(parsedUrl.String(), err)
	}

	// Track HTTP response with actual status code using batching
	metrics.GetGlobalMetricsBatcher().RecordAPIRequest(endpointCategory, fmt.Sprintf("%d", code))

	if code == fiber.StatusOK {
		return body, nil
	}

	// Handle 429 Too Many Requests specifically
	if code == fiber.StatusTooManyRequests {
		return nil, errors.Join(fiber.ErrTooManyRequests, types.NewRateLimitError(parsedUrl.String()))
	}

	if code == fiber.StatusInternalServerError {
		var res ErrorResponse
		if err := json.Unmarshal(body, &res); err != nil {
			return body, err
		}

		if res.Message == "codespace sdk code 26: invalid height: cannot query with height in the future; please provide a valid height" {
			return nil, types.NewInvalidHeightError()
		}
	}

	return nil, types.NewNetworkError(parsedUrl.String(), fmt.Errorf("HTTP %d: %s", code, string(body)))
}

// validateAndParseURL performs security validation and returns parsed URL
func validateAndParseURL(baseUrl, path string) (*url.URL, error) {
	fullURL := fmt.Sprintf("%s%s", baseUrl, path)

	parsedURL, err := url.Parse(fullURL)
	if err != nil {
		return nil, types.NewBadRequestError(fmt.Sprintf("invalid URL format: %s", err.Error()))
	}

	// Check for valid host
	if parsedURL.Host == "" {
		return nil, types.NewBadRequestError("URL must have a valid host")
	}

	return parsedURL, nil
}
