package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"

	"github.com/initia-labs/rollytics/indexer/types"
)

func (s *Scraper) fastSync(client *fiber.Client, height int64, blockChan chan<- types.ScrapedBlock, controlChan <-chan string) int64 {
	var (
		syncedHeight = height - 1
		paused       atomic.Bool
	)

	go func() {
		for signal := range controlChan {
			switch signal {
			case "stop":
				paused.Store(true)
			case "start":
				paused.Store(false)
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return syncedHeight
		default:
		}

		// continue if paused
		if paused.Load() {
			time.Sleep(1 * time.Second)
			continue
		}

		// spin up new goroutine for scraping block with incrementing height
		h := height
		go func(errCount int) {
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

					s.mtx.Lock()
					if block.Height > syncedHeight {
						syncedHeight = block.Height
					}
					s.mtx.Unlock()

					return
				}

				// stop fast syncing after reaching latest height
				if reachedLatestHeight(fmt.Sprintf("%+v", err)) {
					cancel()
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
