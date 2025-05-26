package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/types"
)

const maxRetries = 5

func Get(client *fiber.Client, cfg *config.Config, path string, params map[string]string, headers map[string]string) ([]byte, error) {
	retryCount := 0
	for retryCount <= maxRetries {
		body, err := getRaw(client, cfg, path, params, headers)
		if err == nil {
			return body, nil
		}

		// handle case of querying future height
		if strings.HasPrefix(fmt.Sprintf("%+v", err), "invalid height") {
			time.Sleep(cfg.GetCoolingDuration())
			continue
		}

		retryCount++
		if retryCount > maxRetries {
			return nil, err
		}
		time.Sleep(cfg.GetCoolingDuration())
	}

	return nil, fmt.Errorf("failed to fetch data after %d retries", maxRetries)
}

func getRaw(client *fiber.Client, cfg *config.Config, path string, params map[string]string, headers map[string]string) (body []byte, err error) {
	baseUrl := fmt.Sprintf("%s%s", cfg.GetChainConfig().RestUrl, path)
	parsedUrl, err := url.Parse(baseUrl)
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

	code, body, errs := req.Timeout(5 * time.Second).Bytes()
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

func Post(client *fiber.Client, cfg *config.Config, path string, payload map[string]interface{}, headers map[string]string) ([]byte, error) {
	retryCount := 0
	for retryCount <= maxRetries {
		body, err := postRaw(client, cfg, path, payload, headers)
		if err == nil {
			return body, nil
		}

		// handle case of querying future height
		if strings.HasPrefix(fmt.Sprintf("%+v", err), "invalid height") {
			time.Sleep(cfg.GetCoolingDuration())
			continue
		}

		retryCount++
		if retryCount > maxRetries {
			return nil, err
		}
		time.Sleep(cfg.GetCoolingDuration())
	}

	return nil, fmt.Errorf("failed to post data after %d retries", maxRetries)
}

func postRaw(client *fiber.Client, cfg *config.Config, path string, payload map[string]interface{}, headers map[string]string) (body []byte, err error) {
	baseUrl := fmt.Sprintf("%s%s", cfg.GetChainConfig().JsonRpcUrl, path)
	req := client.Post(baseUrl)

	// set payload
	if payload != nil {
		req = req.JSON(payload)
	}

	// set header
	for key, value := range headers {
		req.Set(key, value)
	}

	code, body, errs := req.Timeout(5 * time.Second).Bytes()
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
