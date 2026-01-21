package querier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
)

type tendermintRpcErrorResponse struct {
	Error struct {
		Code    int64  `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	} `json:"error"`
}

var rpcRotateIndex atomic.Uint32

// FetchRPCWithRotation queries a Tendermint RPC endpoint with rotation.
// It stops rotation when the error indicates a future height.
func FetchRPCWithRotation(ctx context.Context, client *fiber.Client, rpcURLs []string, path string, timeout time.Duration) ([]byte, error) {
	if len(rpcURLs) == 0 {
		return nil, fmt.Errorf("no rpc urls configured")
	}

	start := int(rpcRotateIndex.Add(1)-1) % len(rpcURLs)
	var lastErr error
	for i := 0; i < len(rpcURLs); i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		baseURL := strings.TrimRight(rpcURLs[(start+i)%len(rpcURLs)], "/")
		url := baseURL + path
		body, err := fetchFromRpc(client, timeout, url)
		if err == nil {
			return body, nil
		}
		if isFutureHeightError(err) {
			return nil, err
		}
		lastErr = err
	}

	return nil, lastErr
}

func fetchFromRpc(client *fiber.Client, timeout time.Duration, url string) (body []byte, err error) {
	code, body, errs := client.Get(url).Timeout(timeout).Bytes()
	if err := errors.Join(errs...); err != nil {
		return body, err
	}

	if code == fiber.StatusOK {
		return body, nil
	}

	if code == fiber.StatusInternalServerError {
		var res tendermintRpcErrorResponse
		if err := json.Unmarshal(body, &res); err != nil {
			return body, err
		}

		reHeight := regexp.MustCompile(`current blockchain height (\d+)`)
		heightMatches := reHeight.FindStringSubmatch(res.Error.Data)
		if len(heightMatches) > 1 {
			return body, fmt.Errorf("current height: %s", heightMatches[1])
		}

		reNotFound := regexp.MustCompile(`could not find results for height #(\d+)`)
		notFoundMatches := reNotFound.FindStringSubmatch(res.Error.Data)
		if len(notFoundMatches) > 1 {
			return body, fmt.Errorf("could not find results for height: %s", notFoundMatches[1])
		}
	}

	return body, fmt.Errorf("http response: %d, body: %s", code, string(body))
}

func isFutureHeightError(err error) bool {
	errString := fmt.Sprintf("%+v", err)
	return strings.HasPrefix(errString, "current height") || strings.HasPrefix(errString, "could not find")
}
