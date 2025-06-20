package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/indexer/types"
)

const maxRetries = 5

func Get(client *fiber.Client, coolingDuration time.Duration, baseUrl, path string, params map[string]string, headers map[string]string) ([]byte, error) {
	retryCount := 0
	for retryCount <= maxRetries {
		body, err := getRaw(client, baseUrl, path, params, headers)
		if err == nil {
			return body, nil
		}

		// handle case of querying future height
		if strings.HasPrefix(fmt.Sprintf("%+v", err), "invalid height") {
			time.Sleep(coolingDuration)
			continue
		}

		retryCount++
		if retryCount > maxRetries {
			return nil, err
		}
		time.Sleep(coolingDuration)
	}

	return nil, fmt.Errorf("failed to fetch data after %d retries", maxRetries)
}

func getRaw(client *fiber.Client, baseUrl, path string, params map[string]string, headers map[string]string) (body []byte, err error) {
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

	code, body, errs := req.Timeout(10 * time.Second).Bytes()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	if code != fiber.StatusOK {
		if code == fiber.StatusInternalServerError {
			var res types.ErrorResponse
			if err := json.Unmarshal(body, &res); err != nil {
				return body, err
			}

			if res.Message == "codespace sdk code 26: invalid height: cannot query with height in the future; please provide a valid height" {
				return nil, fmt.Errorf("invalid height")
			}
		}

		return nil, fmt.Errorf("http response: %d, body: %s", code, string(body))
	}

	return body, nil
}

func Post(client *fiber.Client, coolingDuration time.Duration, baseUrl, path string, payload map[string]interface{}, headers map[string]string) ([]byte, error) {
	retryCount := 0
	for retryCount <= maxRetries {
		body, err := postRaw(client, baseUrl, path, payload, headers)
		if err == nil {
			return body, nil
		}

		// handle case of querying future height
		if strings.HasPrefix(fmt.Sprintf("%+v", err), "invalid height") {
			time.Sleep(coolingDuration)
			continue
		}

		retryCount++
		if retryCount > maxRetries {
			return nil, err
		}
		time.Sleep(coolingDuration)
	}

	return nil, fmt.Errorf("failed to post data after %d retries", maxRetries)
}

func postRaw(client *fiber.Client, baseUrl, path string, payload map[string]interface{}, headers map[string]string) (body []byte, err error) {
	req := client.Post(fmt.Sprintf("%s%s", baseUrl, path))

	// set payload
	if payload != nil {
		req = req.JSON(payload)
	}

	// set header
	for key, value := range headers {
		req.Set(key, value)
	}

	code, body, errs := req.Timeout(10 * time.Second).Bytes()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	if code != fiber.StatusOK {
		if code == fiber.StatusInternalServerError {
			var res types.ErrorResponse
			if err := json.Unmarshal(body, &res); err != nil {
				return body, err
			}

			if res.Message == "codespace sdk code 26: invalid height: cannot query with height in the future; please provide a valid height" {
				return nil, fmt.Errorf("invalid height")
			}
		}

		return nil, fmt.Errorf("http response: %d, body: %s", code, string(body))
	}

	return body, nil
}
