package wasm_nft

import (
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
)

func TestIsValidCollectionEvent(t *testing.T) {
	tests := []struct {
		name     string
		attrMap  map[string]string
		expected bool
	}{
		{
			name: "valid event with minter and creator",
			attrMap: map[string]string{
				"minter":  "minter123",
				"creator": "creator456",
			},
			expected: true,
		},
		{
			name: "valid event with minter and owner",
			attrMap: map[string]string{
				"minter": "minter123",
				"owner":  "owner789",
			},
			expected: true,
		},
		{
			name: "valid event with minter, creator and owner",
			attrMap: map[string]string{
				"minter":  "minter123",
				"creator": "creator456",
				"owner":   "owner789",
			},
			expected: true,
		},
		{
			name: "invalid event - missing minter",
			attrMap: map[string]string{
				"creator": "creator456",
				"owner":   "owner789",
			},
			expected: false,
		},
		{
			name: "invalid event - missing creator and owner",
			attrMap: map[string]string{
				"minter": "minter123",
			},
			expected: false,
		},
		{
			name:     "invalid event - empty attrMap",
			attrMap:  map[string]string{},
			expected: false,
		},
		{
			name: "invalid event - only creator",
			attrMap: map[string]string{
				"creator": "creator456",
			},
			expected: false,
		},
		{
			name: "invalid event - only owner",
			attrMap: map[string]string{
				"owner": "owner789",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidCollectionEvent(tt.attrMap)
			if result != tt.expected {
				t.Errorf("isValidCollectionEvent() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestFilterCollectionAddrs(t *testing.T) {
	tests := []struct {
		name          string
		block         indexertypes.ScrapedBlock
		expectedAddrs map[string]bool
		expectedCount int
		description   string
	}{
		{
			name: "single_valid_contract_with_creator",
			block: indexertypes.ScrapedBlock{
				ChainId:   "test-chain",
				Height:    100,
				Timestamp: time.Now(),
				Hash:      "test-hash-1",
				Proposer:  "test-proposer",
				Txs:       []string{"dGVzdC10eA=="},
				TxResults: []abci.ExecTxResult{
					{
						Events: []abci.Event{
							{
								Type: "wasm",
								Attributes: []abci.EventAttribute{
									{Key: "_contract_address", Value: "contract1"},
									{Key: "minter", Value: "minter123"},
									{Key: "creator", Value: "creator456"},
								},
							},
						},
					},
				},
			},
			expectedAddrs: map[string]bool{"contract1": true},
			expectedCount: 1,
			description:   "Single contract with minter and creator should be included",
		},
		{
			name: "multiple_valid_contracts_mixed_attributes",
			block: indexertypes.ScrapedBlock{
				ChainId:   "test-chain",
				Height:    101,
				Timestamp: time.Now(),
				Hash:      "test-hash-2",
				Proposer:  "test-proposer",
				Txs:       []string{"dGVzdC10eA==", "dGVzdC10eA==", "dGVzdC10eA=="},
				TxResults: []abci.ExecTxResult{
					{
						Events: []abci.Event{
							{
								Type: "wasm",
								Attributes: []abci.EventAttribute{
									{Key: "_contract_address", Value: "contract2"},
									{Key: "minter", Value: "minter456"},
									{Key: "creator", Value: "creator789"},
								},
							},
						},
					},
					{
						Events: []abci.Event{
							{
								Type: "wasm",
								Attributes: []abci.EventAttribute{
									{Key: "_contract_address", Value: "contract3"},
									{Key: "minter", Value: "minter101"},
									{Key: "owner", Value: "owner202"},
								},
							},
						},
					},
					{
						Events: []abci.Event{
							{
								Type: "wasm",
								Attributes: []abci.EventAttribute{
									{Key: "_contract_address", Value: "contract4"},
									{Key: "minter", Value: "minter303"},
									{Key: "creator", Value: "creator404"},
									{Key: "owner", Value: "owner505"},
								},
							},
						},
					},
				},
			},
			expectedAddrs: map[string]bool{"contract2": true, "contract3": true, "contract4": true},
			expectedCount: 3,
			description:   "Multiple valid contracts with different attribute combinations should all be included",
		},
		{
			name: "mixed_valid_invalid_events_with_non_wasm",
			block: indexertypes.ScrapedBlock{
				ChainId:   "test-chain",
				Height:    102,
				Timestamp: time.Now(),
				Hash:      "test-hash-3",
				Proposer:  "test-proposer",
				Txs:       []string{"dGVzdC10eA==", "dGVzdC10eA==", "dGVzdC10eA==", "dGVzdC10eA=="},
				TxResults: []abci.ExecTxResult{
					{
						Events: []abci.Event{
							{
								Type: "wasm",
								Attributes: []abci.EventAttribute{
									{Key: "_contract_address", Value: "contract5"},
									{Key: "minter", Value: "minter606"},
									{Key: "creator", Value: "creator707"},
								},
							},
						},
					},
					{
						Events: []abci.Event{
							{
								Type: "wasm",
								Attributes: []abci.EventAttribute{
									{Key: "_contract_address", Value: "contract6"},
									{Key: "minter", Value: "minter808"},
									// Missing creator and owner - should be invalid
								},
							},
						},
					},
					{
						Events: []abci.Event{
							{
								Type: "transfer", // Non-wasm event
								Attributes: []abci.EventAttribute{
									{Key: "_contract_address", Value: "contract7"},
									{Key: "minter", Value: "minter909"},
									{Key: "creator", Value: "creator000"},
								},
							},
						},
					},
					{
						Events: []abci.Event{
							{
								Type: "wasm",
								Attributes: []abci.EventAttribute{
									{Key: "_contract_address", Value: "contract8"},
									{Key: "creator", Value: "creator111"},
									// Missing minter - should be invalid
								},
							},
						},
					},
				},
			},
			expectedAddrs: map[string]bool{"contract5": true},
			expectedCount: 1,
			description:   "Only valid wasm contracts should be included, invalid and non-wasm events filtered out",
		},
		{
			name: "complex_block_with_preblock_beginblock_endblock",
			block: indexertypes.ScrapedBlock{
				ChainId:   "test-chain",
				Height:    103,
				Timestamp: time.Now(),
				Hash:      "test-hash-4",
				Proposer:  "test-proposer",
				PreBlock: []abci.Event{
					{
						Type: "wasm",
						Attributes: []abci.EventAttribute{
							{Key: "_contract_address", Value: "contract9"},
							{Key: "minter", Value: "minter222"},
							{Key: "creator", Value: "creator333"},
						},
					},
				},
				BeginBlock: []abci.Event{
					{
						Type: "wasm",
						Attributes: []abci.EventAttribute{
							{Key: "_contract_address", Value: "contract10"},
							{Key: "minter", Value: "minter444"},
							{Key: "owner", Value: "owner555"},
						},
					},
				},
				EndBlock: []abci.Event{
					{
						Type: "wasm",
						Attributes: []abci.EventAttribute{
							{Key: "_contract_address", Value: "contract11"},
							{Key: "minter", Value: "minter666"},
							{Key: "creator", Value: "creator777"},
							{Key: "owner", Value: "owner888"},
						},
					},
				},
				Txs: []string{"dGVzdC10eA=="},
				TxResults: []abci.ExecTxResult{
					{
						Events: []abci.Event{
							{
								Type: "wasm",
								Attributes: []abci.EventAttribute{
									{Key: "_contract_address", Value: "contract12"},
									{Key: "minter", Value: "minter999"},
									{Key: "creator", Value: "creator000"},
								},
							},
						},
					},
				},
			},
			expectedAddrs: map[string]bool{"contract9": true, "contract10": true, "contract11": true, "contract12": true},
			expectedCount: 4,
			description:   "Events from PreBlock, BeginBlock, EndBlock, and transactions should all be processed",
		},
		{
			name: "edge_cases_empty_and_missing_data",
			block: indexertypes.ScrapedBlock{
				ChainId:   "test-chain",
				Height:    104,
				Timestamp: time.Now(),
				Hash:      "test-hash-5",
				Proposer:  "test-proposer",
				Txs:       []string{"dGVzdC10eA==", "dGVzdC10eA==", "dGVzdC10eA=="},
				TxResults: []abci.ExecTxResult{
					{
						Events: []abci.Event{
							{
								Type: "wasm",
								Attributes: []abci.EventAttribute{
									{Key: "_contract_address", Value: "contract13"},
									{Key: "minter", Value: "minter111"},
									{Key: "creator", Value: "creator222"},
								},
							},
						},
					},
					{
						Events: []abci.Event{
							{
								Type: "wasm",
								Attributes: []abci.EventAttribute{
									{Key: "minter", Value: "minter333"},
									{Key: "creator", Value: "creator444"},
									// Missing _contract_address
								},
							},
						},
					},
					{
						Events: []abci.Event{
							{
								Type:       "wasm",
								Attributes: []abci.EventAttribute{}, // Empty attributes
							},
						},
					},
				},
			},
			expectedAddrs: map[string]bool{"contract13": true},
			expectedCount: 1,
			description:   "Events with missing _contract_address or empty attributes should be ignored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterCollectionAddrs(tt.block)

			// Check the count
			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d contract addresses, got %d. %s",
					tt.expectedCount, len(result), tt.description)
			}

			// Check each expected address
			for expectedAddr := range tt.expectedAddrs {
				if !result[expectedAddr] {
					t.Errorf("Expected contract address %s to be in the result, but it wasn't. %s",
						expectedAddr, tt.description)
				}
			}

			// Check that no unexpected addresses are present
			for resultAddr := range result {
				if !tt.expectedAddrs[resultAddr] {
					t.Errorf("Unexpected contract address %s found in result. %s",
						resultAddr, tt.description)
				}
			}

			// Additional validation: ensure result is not nil
			if result == nil {
				t.Errorf("Result should not be nil. %s", tt.description)
			}
		})
	}
}
