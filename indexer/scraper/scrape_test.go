package scraper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cbjson "github.com/cometbft/cometbft/libs/json"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/metrics"
	"github.com/initia-labs/rollytics/util/querier"
)

func TestScrapeBlockTransactionResultCounts(t *testing.T) {
	metrics.Init("scraper-test")
	tests := []struct {
		name    string
		txs     []string
		results []abci.ExecTxResult
		wantErr string
	}{
		{
			name: "matching results including failed transaction",
			txs:  []string{"AQ==", "Ag=="},
			results: []abci.ExecTxResult{
				{GasUsed: 10, Events: []abci.Event{{Type: "move"}}},
				{Code: 7, GasUsed: 20},
			},
		},
		{name: "empty block with null arrays"},
		{name: "empty block with empty arrays", txs: []string{}, results: []abci.ExecTxResult{}},
		{
			name: "missing results", txs: []string{"AQ=="},
			wantErr: "block 7 has 1 transactions but 0 transaction results",
		},
		{
			name: "too few results", txs: []string{"AQ==", "Ag=="}, results: []abci.ExecTxResult{{}},
			wantErr: "block 7 has 2 transactions but 1 transaction results",
		},
		{
			name: "too many results", txs: []string{"AQ=="}, results: []abci.ExecTxResult{{}, {}},
			wantErr: "block 7 has 1 transactions but 2 transaction results",
		},
		{
			name: "results for empty block", results: []abci.ExecTxResult{{}},
			wantErr: "block 7 has 0 transactions but 1 transaction results",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var block GetBlockResponse
			block.Result.Block.Header.ChainId = "scraper-test"
			block.Result.Block.Header.Height = "7"
			block.Result.Block.Header.Time = "2026-11-18T00:00:00Z"
			block.Result.Block.Header.ProposerAddress = "0102030405060708090A0B0C0D0E0F1011121314"
			block.Result.Block.Data.Txs = tt.txs
			blockBody, err := json.Marshal(block)
			require.NoError(t, err)
			var results GetBlockResultsResponse
			results.Result.TxsResults = tt.results
			resultsBody, err := cbjson.Marshal(results)
			require.NoError(t, err)

			rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/block":
					_, _ = w.Write(blockBody)
				case "/block_results":
					_, _ = w.Write(resultsBody)
				default:
					http.NotFound(w, r)
				}
			}))
			defer rpc.Close()
			client := fiber.AcquireClient()
			defer fiber.ReleaseClient(client)

			scraped, err := scrapeBlock(t.Context(), client, 7, &config.Config{}, &querier.Querier{RpcUrls: []string{rpc.URL}})
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				require.Zero(t, scraped)
				require.False(t, reachedLatestHeight(err.Error()), "a count mismatch must remain a scrape error")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.txs, scraped.Txs)
			require.Len(t, scraped.TxResults, len(tt.results))
			for i, result := range tt.results {
				require.Equal(t, result.Code, scraped.TxResults[i].Code)
				require.Equal(t, result.GasUsed, scraped.TxResults[i].GasUsed)
			}
			// Exercise a consumer that indexes results by transaction position.
			_, err = indexerutil.ExtractEvents(scraped, "move")
			require.NoError(t, err)
		})
	}
}
