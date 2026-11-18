package indexer

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/metrics"
	"github.com/initia-labs/rollytics/types"
)

func TestCollectCancelsBlockedControlSignal(t *testing.T) {
	metrics.Init("control-test")
	for _, paused := range []bool{false, true} {
		name := "pause"
		if paused {
			name = "resume"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			i := &Indexer{
				ctx: ctx, cfg: &config.Config{},
				logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
				controlChan: make(chan string, 1), paused: paused,
			}
			if !paused {
				i.prepareCount = types.MaxInflightBlocks + 1
			}
			// Simulate the scraper listener stopping with an unread signal queued.
			i.controlChan <- "previous"
			done := make(chan struct{})
			go func() {
				defer close(done)
				i.collect()
			}()
			t.Cleanup(func() {
				cancel()
				select {
				case <-i.controlChan:
				default:
				}
				<-done
			})
			require.Eventually(t, func() bool {
				if i.mtx.TryLock() {
					i.mtx.Unlock()
					return false
				}
				return true
			}, time.Second, time.Millisecond)
			cancel()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("collector remained blocked sending a control signal after cancellation")
			}
			require.True(t, i.mtx.TryLock(), "collector must release its mutex on cancellation")
			i.mtx.Unlock()
		})
	}
}
