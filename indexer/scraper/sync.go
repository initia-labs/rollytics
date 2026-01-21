package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"

	"github.com/initia-labs/rollytics/indexer/types"
	commontypes "github.com/initia-labs/rollytics/types"
)

const (
	running int32 = iota
	paused
	stopped
)

func (s *Scraper) fastSync(ctx context.Context, client *fiber.Client, height int64, blockChan chan<- types.ScrapedBlock, controlChan <-chan string) int64 {
	var (
		syncedHeight = height - 1
		status       atomic.Int32
		wg           sync.WaitGroup
	)

	go func() {
		for signal := range controlChan {
			switch signal {
			case "pause":
				if status.Load() == running {
					status.Store(paused)
				}
			case "start":
				if status.Load() == paused {
					status.Store(running)
				}
			}
		}
	}()

	defer func() {
		// wait for all goroutines to finish
		wg.Wait()
	}()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("fastSync() shutting down gracefully")
			wg.Wait()
			return syncedHeight
		default:
		}
		currentState := status.Load()

		// exit if stopped (reached latest height)
		if currentState == stopped {
			wg.Wait()
			return syncedHeight
		}

		// continue if paused
		if currentState == paused {
			time.Sleep(1 * time.Second)
			continue
		}

		// spin up new goroutine for scraping block with incrementing height
		h := height
		wg.Add(1)
		go func(errCount int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				block, err := scrapeBlock(ctx, client, h, s.cfg, s.querier)

				// if no error, cache the scraped block to block map and return
				if err == nil {
					s.logger.Info("scraped block", slog.Int64("height", block.Height))
					blockChan <- block
					s.trackScrapedBlock()

					s.mtx.Lock()
					if block.Height > syncedHeight {
						syncedHeight = block.Height
					}
					s.mtx.Unlock()

					return
				}

				// stop fast syncing after reaching latest height
				if reachedLatestHeight(fmt.Sprintf("%+v", err)) {
					status.Store(stopped)
					return
				}

				s.logger.Info("error while scraping block", slog.Int64("height", h), slog.Any("error", err))
				time.Sleep(s.cfg.GetCoolingDuration())
			}
		}(0)

		height++
		time.Sleep(s.cfg.GetCoolingDuration())
	}
}

func (s *Scraper) slowSync(ctx context.Context, client *fiber.Client, height int64, blockChan chan<- types.ScrapedBlock) {
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("slowSync() shutting down gracefully")
			return
		default:
		}
		var (
			results []ScrapResult
			g       errgroup.Group
		)

		for i := range commontypes.BatchScrapSize {
			h := height + int64(i)
			g.Go(func() error {
				block, err := scrapeBlock(ctx, client, h, s.cfg, s.querier)
				result := ScrapResult{
					Height: h,
					Err:    err,
				}

				s.mtx.Lock()
				results = append(results, result)
				s.mtx.Unlock()

				if err == nil {
					s.logger.Info("scraped block", slog.Int64("height", block.Height))
					blockChan <- block
					s.trackScrapedBlock()
				} else if !reachedLatestHeight(fmt.Sprintf("%+v", err)) {
					// log only if it is not related to reached latest height error
					s.logger.Info("error while scraping block", slog.Int64("height", h), slog.Any("error", err))
				}

				return nil
			})
		}

		if err := g.Wait(); err != nil {
			s.logger.Error("error while scraping blocks", slog.Any("error", err))
			time.Sleep(s.cfg.GetCoolingDuration())
			continue
		}

		sort.Slice(results, func(i, j int) bool {
			return results[i].Height < results[j].Height
		})

		// update height
		var errorHeight int64 = -1
		for _, res := range results {
			if res.Err != nil {
				errorHeight = res.Height
				break
			}
		}
		if errorHeight == -1 {
			height += commontypes.BatchScrapSize
		} else {
			height = errorHeight
		}

		time.Sleep(s.cfg.GetCoolingDuration())
	}
}

func reachedLatestHeight(errString string) bool {
	return strings.HasPrefix(errString, "current height") || strings.HasPrefix(errString, "could not find")
}
