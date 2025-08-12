package util

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/semaphore"

	"github.com/initia-labs/rollytics/config"
)

const (
	maxRetries = 5
)

var limiter *semaphore.Weighted

func InitLimiter(cfg *config.Config) {
	limiter = semaphore.NewWeighted(int64(cfg.GetMaxConcurrentRequests()))
}

type ErrorResponse struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

func Get(client *fiber.Client, coolingDuration, timeout time.Duration, baseUrl, path string, params map[string]string, headers map[string]string) ([]byte, error) {
	retryCount := 0
	for retryCount < maxRetries {
		body, err := getRaw(client, timeout, baseUrl, path, params, headers)
		if err == nil {
			return body, nil
		}

		// handle case of querying future height
		if strings.HasPrefix(fmt.Sprintf("%+v", err), "invalid height") {
			time.Sleep(coolingDuration)
			continue
		}

		retryCount++
		time.Sleep(coolingDuration)
	}

	return nil, fmt.Errorf("failed to fetch data after %d retries", maxRetries)
}

func getRaw(client *fiber.Client, timeout time.Duration, baseUrl, path string, params map[string]string, headers map[string]string) (body []byte, err error) {
	ctx := context.Background()
	if err := limiter.Acquire(ctx, 1); err != nil {
		return nil, fmt.Errorf("failed to acquire semaphore: %w", err)
	}
	defer limiter.Release(1)

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
		return nil, err
	}

	if code == fiber.StatusOK {
		return body, nil
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
	for retryCount < maxRetries {
		body, err := postRaw(client, timeout, baseUrl, path, payload, headers)
		if err == nil {
			return body, nil
		}

		// handle case of querying future height
		if strings.HasPrefix(fmt.Sprintf("%+v", err), "invalid height") {
			time.Sleep(coolingDuration)
			continue
		}

		retryCount++
		time.Sleep(coolingDuration)
	}

	return nil, fmt.Errorf("failed to post data after %d retries", maxRetries)
}

func postRaw(client *fiber.Client, timeout time.Duration, baseUrl, path string, payload map[string]any, headers map[string]string) (body []byte, err error) {
	ctx := context.Background()
	if err := limiter.Acquire(ctx, 1); err != nil {
		return nil, fmt.Errorf("failed to acquire semaphore: %w", err)
	}
	defer limiter.Release(1)

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
		return nil, err
	}

	if code == fiber.StatusOK {
		return body, nil
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
