package da

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// NamespaceForChain derives the namespace ID from a chain ID (first 10 bytes of sha256).
func NamespaceForChain(chainID string) []byte {
	h := sha256.Sum256([]byte(chainID))
	return h[:10]
}

// CelestiaTxSearchResponse is the RPC tx_search result shape.
type CelestiaTxSearchResponse struct {
	Txs        []CelestiaResultTx `json:"txs"`
	TotalCount string             `json:"total_count"`
}

// CelestiaResultTx is one tx in tx_search.
type CelestiaResultTx struct {
	Hash   string `json:"hash"`
	Height string `json:"height"`
	Index  uint32 `json:"index"`
	Tx     string `json:"tx"`
}

// CelestiaRPCRequest / CelestiaRPCResponse for JSON-RPC.
type CelestiaRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type CelestiaRPCResponse struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int                    `json:"id"`
	Result  CelestiaTxSearchResponse `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// CelestiaBlockResponse is the block RPC result.
type CelestiaBlockResponse struct {
	Result struct {
		Block struct {
			Data struct {
				Txs []string `json:"txs"`
			} `json:"data"`
		} `json:"block"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// TxSearch runs tx_search on the Celestia RPC endpoint.
func TxSearch(ctx context.Context, rpcURL, query string, page, perPage int) (*CelestiaTxSearchResponse, error) {
	reqBody := CelestiaRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tx_search",
		Params:  []interface{}{query, false, strconv.Itoa(page), strconv.Itoa(perPage), "asc"},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tx_search status %d: %s", resp.StatusCode, string(b))
	}

	var rpcResp CelestiaRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("tx_search error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return &rpcResp.Result, nil
}

// Block fetches a block at height from the Celestia RPC.
func Block(ctx context.Context, rpcURL string, height int64) (*CelestiaBlockResponse, error) {
	base := strings.TrimRight(rpcURL, "/")
	url := fmt.Sprintf("%s/block?height=%d", base, height)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("block status %d: %s", resp.StatusCode, string(b))
	}

	var blockResp CelestiaBlockResponse
	if err := json.NewDecoder(resp.Body).Decode(&blockResp); err != nil {
		return nil, err
	}
	if blockResp.Error != nil {
		return nil, fmt.Errorf("block error %d: %s", blockResp.Error.Code, blockResp.Error.Message)
	}
	return &blockResp, nil
}

// ExtractBlobsFromTx finds blobs matching namespace in raw tx bytes.
// BlobTx wire format: we look for the 10-byte namespace and then length-prefixed blob data (best-effort).
func ExtractBlobsFromTx(txBytes []byte, namespace []byte) [][]byte {
	var blobs [][]byte
	if len(namespace) != 10 || len(txBytes) < 10 {
		return blobs
	}
	// Scan for namespace occurrence and try to read length-prefixed data after it
	for i := 0; i <= len(txBytes)-10; i++ {
		if !bytes.Equal(txBytes[i:i+10], namespace) {
			continue
		}
		// Often blob data is after namespace with a length prefix (varint or 4 bytes)
		off := i + 10
		if off+4 > len(txBytes) {
			continue
		}
		// Try 4-byte big-endian length
		ln := int(binary.BigEndian.Uint32(txBytes[off : off+4]))
		off += 4
		if ln > 0 && ln <= 10*1024*1024 && off+ln <= len(txBytes) {
			blobs = append(blobs, append([]byte(nil), txBytes[off:off+ln]...))
		}
		// Only use first match per tx to avoid duplicates
		break
	}
	return blobs
}

// RecoverBatchDataFromCelestia fetches all batch blobs from Celestia for the given chain ID.
func RecoverBatchDataFromCelestia(ctx context.Context, rpcURL, chainID, senderAddr string) ([][]byte, error) {
	namespace := NamespaceForChain(chainID)
	query := "message.action='/celestia.blob.v1.MsgPayForBlobs'"
	if senderAddr != "" {
		query += fmt.Sprintf(" AND message.sender='%s'", senderAddr)
	}

	var data [][]byte
	page := 1
	perPage := 100
	var lastHeight int64 = -1
	var lastBlock *CelestiaBlockResponse

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		txsResult, err := TxSearch(ctx, rpcURL, query, page, perPage)
		if err != nil {
			return nil, fmt.Errorf("tx_search: %w", err)
		}

		totalCount, _ := strconv.Atoi(txsResult.TotalCount)

		for _, tx := range txsResult.Txs {
			height, err := strconv.ParseInt(tx.Height, 10, 64)
			if err != nil {
				continue
			}

			if lastBlock == nil || height != lastHeight {
				blockResp, err := Block(ctx, rpcURL, height)
				if err != nil {
					return nil, fmt.Errorf("block %d: %w", height, err)
				}
				lastBlock = blockResp
				lastHeight = height
			}

			if int(tx.Index) >= len(lastBlock.Result.Block.Data.Txs) {
				continue
			}
			txB64 := lastBlock.Result.Block.Data.Txs[tx.Index]
			txBytes, err := base64Decode(txB64)
			if err != nil {
				continue
			}
			blobs := ExtractBlobsFromTx(txBytes, namespace)
			for _, b := range blobs {
				data = append(data, b)
			}
		}

		if totalCount <= page*perPage {
			break
		}
		page++
	}

	return data, nil
}

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// RecoverBlockAtHeight fetches batch blobs from Celestia and returns the block at the given height.
func RecoverBlockAtHeight(ctx context.Context, rpcURL, chainID, senderAddr string, height int64) (*RecoveredBlock, error) {
	blobs, err := RecoverBatchDataFromCelestia(ctx, rpcURL, chainID, senderAddr)
	if err != nil {
		return nil, err
	}
	return RecoverBlockFromBlobs(ctx, blobs, height)
}
