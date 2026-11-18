package cache

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/hashicorp/golang-lru/v2/simplelru"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/types"
)

func TestValidatorCacheExpires(t *testing.T) {
	previous := validatorCache
	var err error
	validatorCache, err = simplelru.NewLRU[string, validatorCacheEntry](2, nil)
	require.NoError(t, err)
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

func TestExpiredValidatorDoesNotEvictValidEntry(t *testing.T) {
	previous := validatorCache
	var err error
	validatorCache, err = simplelru.NewLRU[string, validatorCacheEntry](2, nil)
	require.NoError(t, err)
	t.Cleanup(func() { validatorCache = previous })

	synctest.Test(t, func(t *testing.T) {
		validator := &types.ValidatorResponse{Validator: types.Validator{OperatorAddress: "operator"}}
		SetValidatorCache("expired", validator)
		time.Sleep(validatorCacheTTL / 2)
		SetValidatorCache("valid", validator)
		time.Sleep(validatorCacheTTL / 2)

		_, ok := GetValidatorCache("expired")
		require.False(t, ok)
		SetValidatorCache("new", validator)
		got, ok := GetValidatorCache("valid")
		require.True(t, ok)
		require.Equal(t, validator, got)
	})
}
