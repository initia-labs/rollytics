package block

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cometbft/cometbft/crypto/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
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
			wantRequests := int32(2) // One lookup per response, reused within the list.
			if tc.paginate {
				wantRequests = 4
			}
			require.Equal(t, wantRequests, requests.Load())
			if tc.missing {
				registered.Store(true)
				blocks, err := ToBlocksResponse(ctx, []types.CollectedBlock{cb, cb}, q)
				require.NoError(t, err)
				want := Proposer{Moniker: validator.Moniker, OperatorAddress: sdk.ValAddress(otherKey.Address()).String()}
				require.Equal(t, want, blocks[0].Proposer)
				require.Equal(t, want, blocks[1].Proposer)
				require.Equal(t, wantRequests+1, requests.Load())
			}
		})
	}
}
