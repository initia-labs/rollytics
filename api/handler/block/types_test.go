package block

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cometbft/cometbft/crypto/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/cache"
	"github.com/initia-labs/rollytics/util/querier"
)

func TestBlockProposerLookup(t *testing.T) {
	t.Chdir(t.TempDir())
	for key, value := range map[string]string{
		"VM_TYPE": "wasm", "CHAIN_ID": "test-chain", "DB_DSN": "unused",
		"RPC_URL": "http://localhost:26657", "REST_URL": "http://localhost:1317",
	} {
		t.Setenv(key, value)
	}
	cfg, err := config.GetConfig()
	require.NoError(t, err)
	querier.InitUtil(cfg)
	cache.InitializeCaches(cfg.GetCacheConfig())

	for _, tc := range []struct {
		name     string
		missing  bool
		paginate bool
	}{
		{name: "separate operator address"},
		{name: "match on second page", paginate: true},
		{name: "missing consensus key", missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pubKey := ed25519.GenPrivKey().PubKey()
			proposerAddr := sdk.ValAddress(pubKey.Address()).String()
			otherKey := ed25519.GenPrivKeyFromSecret([]byte("other")).PubKey()
			validator := types.Validator{
				Moniker:         "new sequencer",
				OperatorAddress: sdk.ValAddress(otherKey.Address()).String(),
				ConsensusPubkey: types.ConsensusPubkey{
					Type: "/cosmos.crypto.ed25519.PubKey",
					Key:  base64.StdEncoding.EncodeToString(pubKey.Bytes()),
				},
			}
			if tc.missing {
				// Matching the operator address must not count as a consensus-key match.
				validator.OperatorAddress = proposerAddr
				validator.ConsensusPubkey.Key = base64.StdEncoding.EncodeToString(otherKey.Bytes())
			}
			var requests atomic.Int32
			var registered atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path != "/opinit/opchild/v1/validators" {
					t.Errorf("unexpected validator query: %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				response := types.ValidatorsResponse{Validators: []types.Validator{validator}}
				if registered.Load() {
					response.Validators[0].ConsensusPubkey.Key = base64.StdEncoding.EncodeToString(pubKey.Bytes())
					response.Validators[0].OperatorAddress = sdk.ValAddress(otherKey.Address()).String()
				}
				if tc.paginate && r.URL.Query().Get("pagination.key") == "" {
					response.Validators = nil
					response.Pagination.NextKey = []byte("next")
				} else if tc.paginate {
					if got := r.URL.Query().Get("pagination.key"); got != base64.StdEncoding.EncodeToString([]byte("next")) {
						t.Errorf("unexpected pagination key: %s", got)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			q := &querier.Querier{RestUrls: []string{server.URL}}
			cb := types.CollectedBlock{
				ChainId: "test-chain", Height: 42, Proposer: proposerAddr,
				Hash: []byte{0xab}, TotalFee: []byte(`[{"denom":"uinit","amount":"7"}]`),
				GasUsed: 100, GasWanted: 200, TxCount: 1,
			}
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			got, err := ToBlockResponse(ctx, cb, q)
			require.NoError(t, err)
			require.Equal(t, "42", got.Height)
			require.Equal(t, "AB", got.Hash)
			require.Equal(t, "100", got.GasUsed)
			require.Equal(t, []Fee{{Denom: "uinit", Amount: "7"}}, got.TotalFee)
			if tc.missing {
				require.Equal(t, Proposer{}, got.Proposer)
			} else {
				require.Equal(t, validator.OperatorAddress, got.Proposer.OperatorAddress)
				require.Equal(t, validator.Moniker, got.Proposer.Moniker)
			}
			blocks, err := ToBlocksResponse(ctx, []types.CollectedBlock{cb, cb}, q)
			require.NoError(t, err)
			require.Equal(t, []Block{got, got}, blocks)
			wantRequests := int32(1) // Successful lookups are cached by proposer address.
			if tc.paginate {
				wantRequests = 2
			} else if tc.missing {
				wantRequests = 2 // Reuse the miss within the block-list request.
			}
			require.Equal(t, wantRequests, requests.Load())
			if tc.missing {
				registered.Store(true)
				blocks, err := ToBlocksResponse(ctx, []types.CollectedBlock{cb, cb}, q)
				require.NoError(t, err)
				wantProposer := Proposer{Moniker: validator.Moniker, OperatorAddress: sdk.ValAddress(otherKey.Address()).String()}
				require.Equal(t, wantProposer, blocks[0].Proposer)
				require.Equal(t, wantProposer, blocks[1].Proposer)
				require.Equal(t, wantRequests+1, requests.Load())
			}
		})
	}

	for _, cancelFirst := range []bool{false, true} {
		name := "waiter cancellation"
		if cancelFirst {
			name = "first caller cancellation"
		}
		t.Run(name, func(t *testing.T) {
			proposerAddr := sdk.ValAddress(ed25519.GenPrivKey().PubKey().Address()).String()
			var requests atomic.Int32
			release := make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				<-release
				// A miss is not positively cached, so both callers must share the flight.
				if err := json.NewEncoder(w).Encode(types.ValidatorsResponse{}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			defer unblock()
			q := &querier.Querier{RestUrls: []string{server.URL}}
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			first, cancelFirstCaller := context.WithCancel(ctx)
			defer cancelFirstCaller()
			second, cancelSecondCaller := context.WithCancel(ctx)
			defer cancelSecondCaller()
			firstJoined := &joinedLookupContext{Context: first, joined: make(chan struct{})}
			secondJoined := &joinedLookupContext{Context: second, joined: make(chan struct{})}
			firstResult := make(chan error, 1)
			secondResult := make(chan error, 1)
			lookup := func(ctx context.Context, result chan<- error) {
				validator, err := getValidator(ctx, q, proposerAddr)
				if validator != nil {
					t.Error("expected a proposer miss")
				}
				result <- err
			}
			go lookup(firstJoined, firstResult)
			select {
			case <-firstJoined.joined:
			case <-ctx.Done():
				t.Fatal("first caller did not join the refresh")
			}
			go lookup(secondJoined, secondResult)
			select {
			case <-secondJoined.joined:
			case <-ctx.Done():
				t.Fatal("second caller did not join the refresh")
			}

			canceledResult, activeResult := secondResult, firstResult
			if cancelFirst {
				cancelFirstCaller()
				canceledResult, activeResult = firstResult, secondResult
			} else {
				cancelSecondCaller()
			}
			select {
			case err := <-canceledResult:
				require.ErrorIs(t, err, context.Canceled)
			case <-ctx.Done():
				t.Fatal("canceled caller remained blocked on the refresh")
			}
			unblock()
			select {
			case err := <-activeResult:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal("active caller did not finish")
			}
			require.EqualValues(t, 1, requests.Load())
		})
	}

	t.Run("failed refresh remains retryable", func(t *testing.T) {
		proposerAddr := sdk.ValAddress(ed25519.GenPrivKey().PubKey().Address()).String()
		q := &querier.Querier{}
		_, err := getValidator(t.Context(), q, proposerAddr)
		require.Error(t, err)
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			if err := json.NewEncoder(w).Encode(types.ValidatorsResponse{}); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()
		q.RestUrls = []string{server.URL}
		got, err := getValidator(t.Context(), q, proposerAddr)
		require.NoError(t, err)
		require.Nil(t, got)
		require.EqualValues(t, 1, requests.Load())
	})
}

// Done is evaluated by the caller's select after DoChan has joined the flight.
type joinedLookupContext struct {
	context.Context
	joined chan struct{}
	once   sync.Once
}

func (c *joinedLookupContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.joined) })
	return c.Context.Done()
}
