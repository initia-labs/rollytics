package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/metrics"
)

type syncLogHandler struct{ slow *atomic.Bool }

func (h syncLogHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h syncLogHandler) Handle(_ context.Context, r slog.Record) error {
	if r.Message == "switching to slow syncing" {
		h.slow.Store(true)
	}
	return nil
}
func (h syncLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h syncLogHandler) WithGroup(string) slog.Handler      { return h }

type syncHarness struct {
	blocks  chan types.ScrapedBlock
	control chan string
	done    chan struct{}
	cancel  context.CancelFunc
}

func startSyncTest(t *testing.T, handler func(http.ResponseWriter, *http.Request, int64, bool) bool) *syncHarness {
	t.Helper()
	var slow atomic.Bool
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, err := strconv.ParseInt(r.URL.Query().Get("height"), 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if handler(w, r, h, slow.Load()) {
			return
		}
		if r.URL.Path == "/block_results" {
			_, _ = fmt.Fprint(w, `{"result":{"txs_results":[],"finalize_block_events":[]}}`)
			return
		}
		var block GetBlockResponse
		block.Result.Block.Header.ChainId = "sync-test"
		block.Result.Block.Header.Height = strconv.FormatInt(h, 10)
		block.Result.Block.Header.Time = "2026-11-18T00:00:00Z"
		block.Result.Block.Header.ProposerAddress = "0102030405060708090A0B0C0D0E0F1011121314"
		_ = json.NewEncoder(w).Encode(block)
	}))
	t.Cleanup(rpc.Close)
	t.Setenv("DB_DSN", "postgres://test:test@localhost/test")
	t.Setenv("CHAIN_ID", "sync-test")
	t.Setenv("VM_TYPE", "move")
	t.Setenv("RPC_URL", rpc.URL)
	t.Setenv("REST_URL", rpc.URL)
	t.Setenv("MAX_CONCURRENT_REQUESTS", "3")
	t.Setenv("COOLING_DURATION", "1ms")
	t.Setenv("QUERY_TIMEOUT", "1s")
	cfg, err := config.GetConfig()
	require.NoError(t, err)
	metrics.Init("sync-test")
	ctx, cancel := context.WithCancel(t.Context())
	h := &syncHarness{make(chan types.ScrapedBlock, 64), make(chan string), make(chan struct{}), cancel}
	t.Cleanup(func() {
		cancel()
		select {
		case <-h.done:
		case <-time.After(3 * time.Second):
			t.Error("scraper did not stop after cancellation")
		}
		close(h.control)
	})
	go func() {
		defer close(h.done)
		scraper := New(cfg, slog.New(syncLogHandler{&slow}))
		scraper.querier.RpcUrls = []string{rpc.URL}
		scraper.Run(ctx, 1, h.blocks, h.control)
	}()
	return h
}

func writeSyncHeadError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = fmt.Fprint(w, `{"error":{"code":-32603,"message":"Internal error","data":"current blockchain height 0"}}`)
}

func nextSyncHeight(t *testing.T, h *syncHarness) int64 {
	t.Helper()
	select {
	case b := <-h.blocks:
		return b.Height
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a scraped block")
		return 0
	}
}

func TestSyncRetriesGapAtFastToSlowTransition(t *testing.T) {
	for _, gap := range []int64{1, 2} {
		t.Run(fmt.Sprintf("gap_%d", gap), func(t *testing.T) {
			higherStarted, headReturned := make(chan struct{}), make(chan struct{})
			var higherOnce, headOnce sync.Once
			h := startSyncTest(t, func(w http.ResponseWriter, r *http.Request, height int64, slow bool) bool {
				if !slow && height == 3 {
					higherOnce.Do(func() { close(higherStarted) })
				}
				if !slow && height == gap {
					select {
					case <-higherStarted:
					case <-r.Context().Done():
						return true
					}
					writeSyncHeadError(w)
					headOnce.Do(func() { close(headReturned) })
					return true
				}
				if !slow && height < gap {
					select {
					case <-headReturned:
					case <-r.Context().Done():
					}
				}
				return false
			})
			for want := int64(1); want <= 8; want++ {
				require.Equal(t, want, nextSyncHeight(t, h), "no gaps or duplicate delivery across the transition")
			}
		})
	}
}

func TestSlowSyncDoesNotPublishPastFailedHeight(t *testing.T) {
	var attempts atomic.Int32
	h := startSyncTest(t, func(w http.ResponseWriter, r *http.Request, height int64, slow bool) bool {
		if !slow || (height == 2 && r.URL.Path == "/block" && attempts.Add(1) <= 2) {
			writeSyncHeadError(w)
			return true
		}
		return false
	})
	for want := int64(1); want <= 12; want++ {
		require.Equal(t, want, nextSyncHeight(t, h), "retry must not replay successful suffixes")
	}
	require.GreaterOrEqual(t, attempts.Load(), int32(3))
}

func TestSlowSyncHonorsPauseAndResume(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var requests atomic.Int32
	h := startSyncTest(t, func(w http.ResponseWriter, r *http.Request, height int64, slow bool) bool {
		if !slow {
			writeSyncHeadError(w)
			return true
		}
		requests.Add(1)
		if height == 1 {
			once.Do(func() { close(started) })
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}
		return false
	})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("slow sync did not start")
	}
	// The second receive ensures the listener has applied the first pause.
	for range 2 {
		select {
		case h.control <- "pause":
		case <-time.After(time.Second):
			t.Fatal("pause listener stopped at transition")
		}
	}
	unblock()
	for range 5 {
		nextSyncHeight(t, h)
	}
	count := requests.Load()
	require.Equal(t, int32(10), count, "only the admitted batch's five block/result pairs may be requested")
	require.Never(t, func() bool { return requests.Load() != count }, 50*time.Millisecond, time.Millisecond,
		"paused slow sync must finish its current batch but admit no new batch")
	h.control <- "start"
	require.Equal(t, int64(6), nextSyncHeight(t, h))
	for range 2 {
		h.control <- "pause"
	}
	h.cancel()
	select {
	case <-h.done:
	case <-time.After(time.Second):
		t.Fatal("cancellation must stop paused sync")
	}
}

func TestFastSyncBoundsOutOfOrderWork(t *testing.T) {
	release := make(chan struct{})
	var started atomic.Int32
	h := startSyncTest(t, func(_ http.ResponseWriter, r *http.Request, height int64, _ bool) bool {
		if r.URL.Path == "/block" {
			started.Add(1)
			if height == 1 {
				select {
				case <-release:
				case <-r.Context().Done():
				}
			}
		}
		return false
	})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	require.Eventually(t, func() bool { return started.Load() >= 3 }, time.Second, time.Millisecond)
	require.Never(t, func() bool { return started.Load() > 3 }, 30*time.Millisecond, time.Millisecond,
		"completed higher blocks must retain their concurrency slots until ordered delivery")
	unblock()
	for want := int64(1); want <= 6; want++ {
		require.Equal(t, want, nextSyncHeight(t, h))
	}
}
