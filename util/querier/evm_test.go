package querier

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/util/cache"
)

func TestGetEvmContractByDenomCachedAliases(t *testing.T) {
	cache.InitializeCaches(&config.CacheConfig{
		AccountCacheSize:          16,
		NftCacheSize:              16,
		MsgTypeCacheSize:          16,
		TypeTagCacheSize:          16,
		MoveDenomCacheSize:        16,
		EvmTxHashCacheSize:        16,
		EvmDenomContractCacheSize: 16,
		ValidatorCacheSize:        16,
	})
	const address = "0x1111111111111111111111111111111111111111"
	ibcHash := strings.Repeat("AB", 32)
	tests := []struct {
		canonical string
		aliases   []string
	}{
		{"GAS", []string{"gas", "Gas", "GAS"}},
		{"ibc/" + ibcHash, []string{"ibc/" + strings.ToLower(ibcHash), "ibc/" + ibcHash}},
		{"evm/Token", []string{"evm/Token"}},
		{"l2/token", []string{"l2/token"}},
	}
	// No endpoints: cached aliases must resolve without an upstream request.
	q := &Querier{}
	for _, tt := range tests {
		cache.SetEvmDenomContractCache(tt.canonical, address)
		for _, alias := range tt.aliases {
			t.Run(alias, func(t *testing.T) {
				got, err := q.GetEvmContractByDenom(t.Context(), alias)
				require.NoError(t, err)
				require.Equal(t, address, got)
			})
		}
	}

	got, err := q.GetEvmContractByDenom(t.Context(), address)
	require.NoError(t, err)
	require.Equal(t, address, got)
}
