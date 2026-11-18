package cache

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/cache"
	"github.com/initia-labs/rollytics/types"
)

func TestValidatorCacheExpires(t *testing.T) {
	previous := validatorCache
	validatorCache = cache.New[string, validatorCacheEntry](2)
	t.Cleanup(func() { validatorCache = previous })

	synctest.Test(t, func(t *testing.T) {
		const proposer = "proposer"
		old := &types.ValidatorResponse{Validator: types.Validator{OperatorAddress: "old-operator"}}
		SetValidatorCache(proposer, old)
		got, ok := GetValidatorCache(proposer)
		require.True(t, ok)
		require.Equal(t, old, got)

		// Virtual time advances without waiting a real minute.
		time.Sleep(validatorCacheTTL)
		got, ok = GetValidatorCache(proposer)
		require.False(t, ok)
		require.Nil(t, got)

		// A missing validator cannot revive the expired entry.
		SetValidatorCache(proposer, nil)
		_, ok = GetValidatorCache(proposer)
		require.False(t, ok)

		updated := &types.ValidatorResponse{Validator: types.Validator{OperatorAddress: "new-operator"}}
		SetValidatorCache(proposer, updated)
		got, ok = GetValidatorCache(proposer)
		require.True(t, ok)
		require.Equal(t, updated, got)
	})
}
