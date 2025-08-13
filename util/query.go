package util

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/semaphore"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/metrics"
)

const (
	maxRetries         = 5
	baseBackoffDelay   = 1 * time.Second
	maxBackoffDelay    = 30 * time.Second
	backoffMultiplier  = 2.0
	jitterFactor       = 0.1
)

var (
	limiter    *semaphore.Weighted
	jitterSeed uint32 = uint32(time.Now().UnixNano())
)

func InitLimiter(cfg *config.Config) {
	limiter = semaphore.NewWeighted(int64(cfg.GetMaxConcurrentRequests()))
}

// simpleRandom generates pseudo-random number using Linear Congruential Generator
// without external dependencies
func simpleRandom() float64 {
	// LCG with standard constants (used by glibc)
	jitterSeed = jitterSeed*1103515245 + 12345
	// Return value between 0.0 and 1.0
	return float64(jitterSeed&0x7FFFFFFF) / float64(0x7FFFFFFF)
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

func Get(client *fiber.Client, coolingDuration, timeout time.Duration, baseUrl, path string, params map[string]string, headers map[string]string) ([]byte, error) {
	retryCount := 0
	rateLimitRetries := 0
	var lastErr error
	
	for retryCount < maxRetries {
		body, err := getRaw(client, timeout, baseUrl, path, params, headers)
		if err == nil {
			return body, nil
		}
		
		lastErr = err

		// handle case of querying future height
		if strings.HasPrefix(fmt.Sprintf("%+v", err), "invalid height") {
			time.Sleep(coolingDuration)
			continue
		}

		// handle 429 Too Many Requests with exponential backoff
		// This doesn't count against regular retry limit to allow for rate limit recovery
		if errors.Is(err, fiber.ErrTooManyRequests) {
			rateLimitRetries++
			
			// Track rate limit hits
			metrics.RateLimitHitsTotal().WithLabelValues(fmt.Sprintf("%s%s", baseUrl, path)).Inc()
			
			backoffDelay := calculateBackoffDelay(rateLimitRetries)
			time.Sleep(backoffDelay)
			continue
		}

		retryCount++
		time.Sleep(coolingDuration)
	}

	// Return the original unwrapped error, not a retry count error
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("failed to fetch data after %d retries", maxRetries)
}

func getRaw(client *fiber.Client, timeout time.Duration, baseUrl, path string, params map[string]string, headers map[string]string) (body []byte, err error) {
	start := time.Now()
	endpoint := fmt.Sprintf("%s%s", baseUrl, path)
	
	// Track concurrent requests
	metrics.ConcurrentRequestsActive().Inc()
	defer func() {
		metrics.ConcurrentRequestsActive().Dec()
		// Record API latency
		duration := time.Since(start).Seconds()
		metrics.ExternalAPILatency().WithLabelValues(endpoint).Observe(duration)
	}()
	
	// Track semaphore wait time
	semaphoreStart := time.Now()
	ctx := context.Background()
	if err := limiter.Acquire(ctx, 1); err != nil {
		return nil, fmt.Errorf("failed to acquire semaphore: %w", err)
	}
	defer limiter.Release(1)
	
	// Record semaphore wait duration
	semaphoreWaitTime := time.Since(semaphoreStart).Seconds()
	metrics.SemaphoreWaitDuration().Observe(semaphoreWaitTime)

	parsedUrl, err := url.Parse(fmt.Sprintf("%s%s", baseUrl, path))
	if err != nil {
		return nil, err
	}

	// set query params
	if params != nil {
		query := parsedUrl.Query()
		for key, value := range params {
			query.Set(key, value)
		}
		parsedUrl.RawQuery = query.Encode()
	}

	req := client.Get(parsedUrl.String())

	// set header
	for key, value := range headers {
		req.Set(key, value)
	}

	code, body, errs := req.Timeout(timeout).Bytes()
	if err := errors.Join(errs...); err != nil {
		// Track network/timeout errors
		metrics.ExternalAPIRequestsTotal().WithLabelValues(endpoint, "error").Inc()
		return nil, err
	}

	// Track HTTP response with actual status code
	metrics.ExternalAPIRequestsTotal().WithLabelValues(endpoint, fmt.Sprintf("%d", code)).Inc()

	if code == fiber.StatusOK {
		return body, nil
	}

	// Handle 429 Too Many Requests specifically
	if code == fiber.StatusTooManyRequests {
		return nil, errors.Join(fiber.ErrTooManyRequests, fmt.Errorf("body: %s", string(body)))
	}

	if code == fiber.StatusInternalServerError {
		var res ErrorResponse
		if err := json.Unmarshal(body, &res); err != nil {
			return body, err
		}

		if res.Message == "codespace sdk code 26: invalid height: cannot query with height in the future; please provide a valid height" {
			return nil, fmt.Errorf("invalid height")
		}
	}

	return nil, fmt.Errorf("http response: %d, body: %s", code, string(body))
}

func Post(client *fiber.Client, coolingDuration, timeout time.Duration, baseUrl, path string, payload map[string]any, headers map[string]string) ([]byte, error) {
	retryCount := 0
	rateLimitRetries := 0
	var lastErr error
	
	for retryCount < maxRetries {
		body, err := postRaw(client, timeout, baseUrl, path, payload, headers)
		if err == nil {
			return body, nil
		}
		
		lastErr = err

		// handle case of querying future height
		if strings.HasPrefix(fmt.Sprintf("%+v", err), "invalid height") {
			time.Sleep(coolingDuration)
			continue
		}

		// handle 429 Too Many Requests with exponential backoff
		// This doesn't count against regular retry limit to allow for rate limit recovery
		if errors.Is(err, fiber.ErrTooManyRequests) {
			rateLimitRetries++
			
			// Track rate limit hits
			metrics.RateLimitHitsTotal().WithLabelValues(fmt.Sprintf("%s%s", baseUrl, path)).Inc()
			
			backoffDelay := calculateBackoffDelay(rateLimitRetries)
			time.Sleep(backoffDelay)
			continue
		}

		retryCount++
		time.Sleep(coolingDuration)
	}

	// Return the original unwrapped error, not a retry count error
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("failed to post data after %d retries", maxRetries)
}

func postRaw(client *fiber.Client, timeout time.Duration, baseUrl, path string, payload map[string]any, headers map[string]string) (body []byte, err error) {
	start := time.Now()
	endpoint := fmt.Sprintf("%s%s", baseUrl, path)
	
	// Track concurrent requests
	metrics.ConcurrentRequestsActive().Inc()
	defer func() {
		metrics.ConcurrentRequestsActive().Dec()
		// Record API latency
		duration := time.Since(start).Seconds()
		metrics.ExternalAPILatency().WithLabelValues(endpoint).Observe(duration)
	}()
	
	// Track semaphore wait time
	semaphoreStart := time.Now()
	ctx := context.Background()
	if err := limiter.Acquire(ctx, 1); err != nil {
		return nil, fmt.Errorf("failed to acquire semaphore: %w", err)
	}
	defer limiter.Release(1)
	
	// Record semaphore wait duration
	semaphoreWaitTime := time.Since(semaphoreStart).Seconds()
	metrics.SemaphoreWaitDuration().Observe(semaphoreWaitTime)

	req := client.Post(fmt.Sprintf("%s%s", baseUrl, path))

	// set payload
	if payload != nil {
		req = req.JSON(payload)
	}

	// set header
	for key, value := range headers {
		req.Set(key, value)
	}

	code, body, errs := req.Timeout(timeout).Bytes()
	if err := errors.Join(errs...); err != nil {
		// Track network/timeout errors
		metrics.ExternalAPIRequestsTotal().WithLabelValues(endpoint, "error").Inc()
		return nil, err
	}

	// Track HTTP response with actual status code
	metrics.ExternalAPIRequestsTotal().WithLabelValues(endpoint, fmt.Sprintf("%d", code)).Inc()

	if code == fiber.StatusOK {
		return body, nil
	}

	// Handle 429 Too Many Requests specifically
	if code == fiber.StatusTooManyRequests {
		return nil, errors.Join(fiber.ErrTooManyRequests, fmt.Errorf("body: %s", string(body)))
	}

	if code == fiber.StatusInternalServerError {
		var res ErrorResponse
		if err := json.Unmarshal(body, &res); err != nil {
			return body, err
		}

		if res.Message == "codespace sdk code 26: invalid height: cannot query with height in the future; please provide a valid height" {
			return nil, fmt.Errorf("invalid height")
		}
	}

	return nil, fmt.Errorf("http response: %d, body: %s", code, string(body))
}
