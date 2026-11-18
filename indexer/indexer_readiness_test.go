package indexer

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/metrics"
	"github.com/initia-labs/rollytics/util/querier"
)

func readinessIndexer(t *testing.T, height string, beforeResponse func()) (*Indexer, <-chan struct{}) {
	t.Helper()
	responded := make(chan struct{})
	var once sync.Once
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if beforeResponse != nil {
			beforeResponse()
		}
		_, _ = fmt.Fprintf(w, `{"block":{"header":{"height":%q}}}`, height)
		once.Do(func() { close(responded) })
	}))
	t.Cleanup(rpc.Close)
	t.Setenv("VM_TYPE", "move")
	t.Setenv("CHAIN_ID", "readiness-test")
	t.Setenv("RPC_URL", rpc.URL)
	t.Setenv("REST_URL", rpc.URL)
	t.Setenv("DB_DSN", "postgres://test:test@localhost/test")
	cfg, err := config.GetConfig()
	require.NoError(t, err)
	querier.InitUtil(cfg)
	metrics.Init("readiness-test")
	return &Indexer{
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		querier: &querier.Querier{RestUrls: []string{rpc.URL}},
	}, responded
}

func TestRunCancelsBeforeReadiness(t *testing.T) {
	for _, tc := range []struct{ name, height string }{
		{"chain at readiness threshold", "10"},
		{"height query fails to parse", "invalid-height"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i, responded := readinessIndexer(t, tc.height, nil)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- i.Run(ctx) }()
			select {
			case <-responded:
			case <-time.After(time.Second):
				t.Fatal("readiness request was not made")
			}
			cancel()
			select {
			case err := <-done:
				require.NoError(t, err, "cancellation is a graceful shutdown, including before readiness")
			case <-time.After(time.Second):
				t.Fatal("Run did not stop during readiness polling")
			}
			// No DB or worker dependencies are supplied: cancellation must return
			// before startup reaches them.
		})
	}
}

func TestReadinessAcceptsReadyChain(t *testing.T) {
	i, _ := readinessIndexer(t, "11", nil)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	i.ctx = ctx
	require.True(t, i.wait())
}

func TestRunAlreadyCanceledBeforeReadiness(t *testing.T) {
	i, requested := readinessIndexer(t, "11", nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	done := make(chan error, 1)
	go func() { done <- i.Run(ctx) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Run did not stop with an already canceled context")
	}
	select {
	case <-requested:
		t.Fatal("already canceled startup must not issue a readiness request")
	default:
	}
}

func TestReadinessDoesNotAcceptResponseAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	i, _ := readinessIndexer(t, "11", cancel)
	i.ctx = ctx
	require.False(t, i.wait(), "a readiness response arriving after shutdown must not advance startup")
}
