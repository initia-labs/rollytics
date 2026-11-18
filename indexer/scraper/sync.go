package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/indexer/types"
	commontypes "github.com/initia-labs/rollytics/types"
)

func (s *Scraper) fastSync(ctx context.Context, client *fiber.Client, height int64, blockChan chan<- types.ScrapedBlock, paused *atomic.Bool) int64 {
	var (
		syncedHeight = height - 1
		stopped      atomic.Bool
		wg           sync.WaitGroup
	)

	// Limit the number of concurrent scraping goroutines.
	// This prevents unbounded goroutine growth when downstream (prepare/collect/DB) is slow.
	maxConc := s.cfg.GetMaxConcurrentRequests()
	if maxConc < 1 {
		maxConc = 1
	}
	sem := make(chan struct{}, maxConc)

	// Keep each worker's slot until its predecessor has finished delivery.
	// This bounds buffered work and prevents publishing past a missing height.
	previous := make(chan struct{})
	close(previous)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("fastSync() shutting down gracefully")
			wg.Wait()
			return syncedHeight
		default:
		}
		// exit if stopped (reached latest height)
		if stopped.Load() {
			wg.Wait()
			return syncedHeight
		}

		// continue if paused
		if paused.Load() {
			select {
			case <-ctx.Done():
			case <-time.After(s.cfg.GetCoolingDuration()):
			}
			continue
		}

		// spin up new goroutine for scraping block with incrementing height
		h := height
		select {
		case sem <- struct{}{}:
			// acquired a slot; safe to spawn and account in wg
		case <-ctx.Done():
			s.logger.Info("fastSync() shutting down gracefully")
			wg.Wait()
			return syncedHeight
		}
		// A pause or head response may arrive while waiting for a slot.
		if stopped.Load() || paused.Load() {
			<-sem
			continue
		}

		before, done := previous, make(chan struct{})
		previous = done
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			defer close(done)
			// Every exit must wait for the predecessor, including a head error,
			// so later workers cannot overtake an earlier unfinished height.
			defer func() { <-before }()

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				block, err := scrapeBlock(ctx, client, h, s.cfg, s.querier)

				// Publish successes in order, stopping at the first missing height.
				if err == nil {
					select {
					case <-before:
					case <-ctx.Done():
						return
					}
					if h != syncedHeight+1 {
						return
					}
					s.logger.Info("scraped block", slog.Int64("height", block.Height))
					select {
					case blockChan <- block:
					case <-ctx.Done():
						return
					}
					s.trackScrapedBlock()

					syncedHeight = h

					return
				}

				// stop fast syncing after reaching latest height
				if reachedLatestHeight(fmt.Sprintf("%+v", err)) {
					stopped.Store(true)
					return
				}

				s.logger.Info("error while scraping block", slog.Int64("height", h), slog.Any("error", err))
				select {
				case <-ctx.Done():
					return
				case <-time.After(s.cfg.GetCoolingDuration()):
				}
			}
		}()

		height++
		select {
		case <-ctx.Done():
		case <-time.After(s.cfg.GetCoolingDuration()):
		}
	}
}

func (s *Scraper) slowSync(ctx context.Context, client *fiber.Client, height int64, blockChan chan<- types.ScrapedBlock, paused *atomic.Bool) {
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("slowSync() shutting down gracefully")
			return
		default:
		}
		// Finish an admitted batch, but do not start another while paused.
		if paused.Load() {
			select {
			case <-ctx.Done():
			case <-time.After(s.cfg.GetCoolingDuration()):
			}
			continue
		}
		var results [commontypes.BatchScrapSize]struct {
			block types.ScrapedBlock
			err   error
		}
		var wg sync.WaitGroup

		for i := range commontypes.BatchScrapSize {
			h := height + int64(i)
			wg.Add(1)
			go func() {
				defer wg.Done()
				block, err := scrapeBlock(ctx, client, h, s.cfg, s.querier)
				results[i].block, results[i].err = block, err

				if err == nil {
					s.logger.Info("scraped block", slog.Int64("height", block.Height))
				} else if !reachedLatestHeight(fmt.Sprintf("%+v", err)) {
					// log only if it is not related to reached latest height error
					s.logger.Info("error while scraping block", slog.Int64("height", h), slog.Any("error", err))
				}
			}()
		}

		wg.Wait()

		// Deliver only the contiguous prefix; retry from the first failure.
		// Successful higher blocks have not been delivered and cannot be replayed.
		for _, res := range results {
			if res.err != nil {
				break
			}
			select {
			case blockChan <- res.block:
				s.trackScrapedBlock()
				height++
			case <-ctx.Done():
				return
			}
		}

		select {
		case <-ctx.Done():
		case <-time.After(s.cfg.GetCoolingDuration()):
		}
	}
}

func reachedLatestHeight(errString string) bool {
	return strings.HasPrefix(errString, "current height") || strings.HasPrefix(errString, "could not find")
}
