package internaltx_test

import (
	"testing"

	"github.com/initia-labs/rollytics/indexer/extension/internaltx"
	"github.com/initia-labs/rollytics/types"
	"github.com/stretchr/testify/require"
)

func TestGrepAddressesFromEvmInternalTx(t *testing.T) {
	tests := []struct {
		name     string
		tx       types.EvmInternalTx
		expected []string
	}{
		{
			name: "basic call with proper input",
			tx: types.EvmInternalTx{
				Type:   "CALL",
				From:   "0x1234567890123456789012345678901234567890",
				To:     "0x0987654321098765432109876543210987654321",
				Input:  "0x12345678000000000000000000000000111111111111111111111111111111111111111100000000000000000000000022222222222222222222222222222222222222220000000000000000000000003333333333333333333333333333333333333333",
				Output: "0x87654321",
			},
			expected: []string{
				"cosmos1zg69v7yszg69v7yszg69v7yszg69v7ys4mp2q5",
				"cosmos1pxrk2seppxrk2seppxrk2seppxrk2sep0yx7a5",
			},
		},
		{
			name: "empty input",
			tx: types.EvmInternalTx{
				Type:   "CALL",
				From:   "0x1234567890123456789012345678901234567890",
				To:     "0x0987654321098765432109876543210987654321",
				Input:  "0x",
				Output: "0x87654321",
			},
			expected: []string{
				"cosmos1zg69v7yszg69v7yszg69v7yszg69v7ys4mp2q5",
				"cosmos1pxrk2seppxrk2seppxrk2seppxrk2sep0yx7a5",
			},
		},
		{
			name: "short input",
			tx: types.EvmInternalTx{
				Type:   "CALL",
				From:   "0x1234567890123456789012345678901234567890",
				To:     "0x0987654321098765432109876543210987654321",
				Input:  "0x12345678",
				Output: "0x87654321",
			},
			expected: []string{
				"cosmos1zg69v7yszg69v7yszg69v7yszg69v7ys4mp2q5",
				"cosmos1pxrk2seppxrk2seppxrk2seppxrk2sep0yx7a5",
			},
		},
		{
			name: "delegatecall with zero value",
			tx: types.EvmInternalTx{
				Type:   "DELEGATECALL",
				From:   "0x1234567890123456789012345678901234567890",
				To:     "0x2222222222222222222222222222222222222222",
				Input:  "0xdeadbeef00000000000000000000000077777777777777777777777777777777777777770000000000000000000000008888888888888888888888888888888888888888000000000000000000000000999999999999999999999999999999999999999",
				Output: "0xbeefdead",
			},
			expected: []string{
				"cosmos1zg69v7yszg69v7yszg69v7yszg69v7ys4mp2q5",
				"cosmos1yg3zyg3zyg3zyg3zyg3zyg3zyg3zyg3zwqjy6c",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := internaltx.GrepAddressesFromEvmInternalTx(tt.tx)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expected, result)
		})
	}
}
