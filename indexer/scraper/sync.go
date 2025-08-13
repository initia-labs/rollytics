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
)

const (
	running int32 = iota
	paused
	stopped
)

func (s *Scraper) fastSync(client *fiber.Client, height int64, blockChan chan<- types.ScrapedBlock, controlChan <-chan string) int64 {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		// wait for all goroutines to finish
		wg.Wait()
	}()

	for {
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

				block, err := scrapeBlock(client, h, s.cfg)

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

				// retry until max err count
				if errCount += 1; errCount > maxErrCount {
					s.logger.Error("failed to scrap block", slog.Int64("height", h), slog.Any("error", err))
					panic(err)
				}

				s.logger.Info("error while scraping block", slog.Int64("height", h), slog.Any("error", err))
				time.Sleep(s.cfg.GetCoolingDuration())
			}
		}(0)

		height++
		time.Sleep(s.cfg.GetCoolingDuration())
	}
}

func (s *Scraper) slowSync(client *fiber.Client, height int64, blockChan chan<- types.ScrapedBlock) {
	for {
		var (
			results []ScrapResult
			g       errgroup.Group
		)

		for i := range batchScrapSize {
			h := height + int64(i)
			g.Go(func() error {
				block, err := scrapeBlock(client, h, s.cfg)
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
			height += batchScrapSize
		} else {
			height = errorHeight
		}

		time.Sleep(s.cfg.GetCoolingDuration())
	}
}

func reachedLatestHeight(errString string) bool {
	return strings.HasPrefix(errString, "current height") || strings.HasPrefix(errString, "could not find")
}
