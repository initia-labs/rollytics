package util_test

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/tmhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wasm_nft "github.com/initia-labs/rollytics/indexer/collector/wasm-nft"
	"github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/indexer/util"
)

func TestExtractEvents(t *testing.T) {
	// Create a simple test transaction
	txData := []byte("test transaction")
	txHash := fmt.Sprintf("%X", tmhash.Sum(txData))

	block := types.ScrapedBlock{
		ChainId:   "test-chain",
		Height:    100,
		Timestamp: time.Now(),
		Hash:      "test-hash",
		Proposer:  "test-proposer",
		Txs: []string{
			base64.StdEncoding.EncodeToString(txData),
		},
		TxResults: []abci.ExecTxResult{
			{
				Events: []abci.Event{
					{
						Type: "transfer",
						Attributes: []abci.EventAttribute{
							{Key: "amount", Value: "100"},
							{Key: "from", Value: "alice"},
						},
					},
					{
						Type: "mint",
						Attributes: []abci.EventAttribute{
							{Key: "amount", Value: "50"},
						},
					},
				},
			},
		},
		PreBlock: []abci.Event{},
		BeginBlock: []abci.Event{
			{
				Type: "transfer",
				Attributes: []abci.EventAttribute{
					{Key: "amount", Value: "200"},
				},
			},
		},
		EndBlock: []abci.Event{},
	}

	t.Run("extract transfer events", func(t *testing.T) {
		events, err := util.ExtractEvents(block, "transfer")
		require.NoError(t, err)
		assert.Len(t, events, 2) // 1 from BeginBlock, 1 from transaction

		// Check that we have events from both BeginBlock and transaction
		hasBeginBlockEvent := false
		hasTxEvent := false

		for _, event := range events {
			assert.Equal(t, "transfer", event.Type)
			if event.TxHash == "" {
				hasBeginBlockEvent = true
				assert.Equal(t, "200", event.AttrMap["amount"])
			} else {
				hasTxEvent = true
				assert.Equal(t, txHash, event.TxHash)
				assert.Equal(t, "100", event.AttrMap["amount"])
				assert.Equal(t, "alice", event.AttrMap["from"])
			}
		}

		assert.True(t, hasBeginBlockEvent, "Should have BeginBlock event")
		assert.True(t, hasTxEvent, "Should have transaction event")
	})

	t.Run("extract mint events", func(t *testing.T) {
		events, err := util.ExtractEvents(block, "mint")
		require.NoError(t, err)
		assert.Len(t, events, 1) // Only from transaction

		event := events[0]
		assert.Equal(t, "mint", event.Type)
		assert.Equal(t, txHash, event.TxHash)
		assert.Equal(t, "50", event.AttrMap["amount"])
	})

	t.Run("extract non-existent events", func(t *testing.T) {
		events, err := util.ExtractEvents(block, "burn")
		require.NoError(t, err)
		assert.Len(t, events, 0)
	})
}

func TestExtractEvents_Base64DecodeError(t *testing.T) {
	block := types.ScrapedBlock{
		ChainId:   "test-chain",
		Height:    100,
		Timestamp: time.Now(),
		Hash:      "test-hash",
		Proposer:  "test-proposer",
		Txs: []string{
			"invalid-base64-string!", // This will cause base64 decoding to fail
		},
		TxResults: []abci.ExecTxResult{
			{
				Events: []abci.Event{
					{
						Type: "transfer",
						Attributes: []abci.EventAttribute{
							{Key: "amount", Value: "100"},
						},
					},
				},
			},
		},
		PreBlock:   []abci.Event{},
		BeginBlock: []abci.Event{},
		EndBlock:   []abci.Event{},
	}

	events, err := util.ExtractEvents(block, "transfer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "illegal base64 data")
	assert.Len(t, events, 0) // Should return empty events when error occurs
}

func TestWasmEventMatcher(t *testing.T) {
	// Create a test block with WASM events
	txData := []byte("test wasm transaction")
	txHash := fmt.Sprintf("%X", tmhash.Sum(txData))

	block := types.ScrapedBlock{
		ChainId:   "test-chain",
		Height:    200,
		Timestamp: time.Now(),
		Hash:      "test-hash-2",
		Proposer:  "test-proposer",
		Txs: []string{
			base64.StdEncoding.EncodeToString(txData),
		},
		TxResults: []abci.ExecTxResult{
			{
				Events: []abci.Event{
					{
						Type: "wasm",
						Attributes: []abci.EventAttribute{
							{Key: "action", Value: "mint"},
							{Key: "token_id", Value: "1"},
						},
					},
					{
						Type: "wasm-cw721",
						Attributes: []abci.EventAttribute{
							{Key: "action", Value: "transfer"},
							{Key: "token_id", Value: "2"},
						},
					},
					{
						Type: "wasm-cw1155",
						Attributes: []abci.EventAttribute{
							{Key: "action", Value: "burn"},
							{Key: "token_id", Value: "3"},
						},
					},
					{
						Type: "transfer", // Non-WASM event
						Attributes: []abci.EventAttribute{
							{Key: "amount", Value: "100"},
						},
					},
				},
			},
		},
		PreBlock:   []abci.Event{},
		BeginBlock: []abci.Event{},
		EndBlock:   []abci.Event{},
	}

	t.Run("extract WASM events with WasmEventMatcher using CustomContractEventPrefix", func(t *testing.T) {
		// Use the same pattern as in collect.go
		events, err := util.ExtractEventsWithMatcher(block, wasm_nft.CustomContractEventPrefix, wasm_nft.WasmEventMatcher)
		require.NoError(t, err)
		assert.Len(t, events, 3) // 3 WASM events (wasm, wasm-cw721, wasm-cw1155)

		// Check that we have the correct WASM events
		eventTypes := make(map[string]bool)
		for _, event := range events {
			assert.Equal(t, txHash, event.TxHash)
			eventTypes[event.Event.Type] = true
		}

		assert.True(t, eventTypes["wasm"], "Should have legacy wasm event")
		assert.True(t, eventTypes["wasm-cw721"], "Should have wasm-cw721 event")
		assert.True(t, eventTypes["wasm-cw1155"], "Should have wasm-cw1155 event")
	})

	t.Run("test WasmEventMatcher function directly", func(t *testing.T) {
		// Test exact match for legacy "wasm" events
		assert.True(t, wasm_nft.WasmEventMatcher("wasm", wasm_nft.CustomContractEventPrefix))

		// Test prefix match for new format
		assert.True(t, wasm_nft.WasmEventMatcher("wasm-cw721", wasm_nft.CustomContractEventPrefix))
		assert.True(t, wasm_nft.WasmEventMatcher("wasm-cw1155", wasm_nft.CustomContractEventPrefix))
		assert.True(t, wasm_nft.WasmEventMatcher("wasm-custom", wasm_nft.CustomContractEventPrefix))

		// Test non-matches
		assert.False(t, wasm_nft.WasmEventMatcher("transfer", wasm_nft.CustomContractEventPrefix))
		assert.False(t, wasm_nft.WasmEventMatcher("mint", wasm_nft.CustomContractEventPrefix))
		assert.False(t, wasm_nft.WasmEventMatcher("cosmos.bank.v1beta1.EventTransfer", wasm_nft.CustomContractEventPrefix))
	})
}
