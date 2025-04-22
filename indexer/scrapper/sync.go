package scrapper

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

func (s *Scrapper) fastSync(client *fiber.Client, height int64) int64 {
	var (
		syncedHeight = height
		mtx          sync.Mutex
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return syncedHeight
		default:
		}

		if s.GetBlockMapSize() > 100 {
			time.Sleep(1 * time.Second)
			continue
		}

		// spin up new goroutine for scrapping block with incrementing height
		h := height
		go func(errCount int) {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				block, err := scrapBlock(client, h, s.cfg)

				// if no error, cache the scrapped block to block map and return
				if err == nil {
					s.logger.Info("scrapped block", slog.Int64("height", block.Height))
					s.SetBlock(block)

					mtx.Lock()
					if block.Height > syncedHeight {
						syncedHeight = block.Height
					}
					mtx.Unlock()

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

				s.logger.Info("error while scrapping block", slog.Int64("height", h), slog.Any("error", err))
				time.Sleep(s.cfg.GetCoolingDuration())
			}
		}(0)

		height++
		time.Sleep(s.cfg.GetCoolingDuration())
	}
}

func (s *Scrapper) slowSync(client *fiber.Client, height int64) {
	for {
		var (
			results []ScrapResult
			g       errgroup.Group
			mtx     sync.Mutex
		)

		for i := range batchScrapSize {
			h := height + int64(i)
			g.Go(func() error {
				block, err := scrapBlock(client, h, s.cfg)
				result := ScrapResult{
					Height: h,
					Err:    err,
				}

				mtx.Lock()
				results = append(results, result)
				mtx.Unlock()

				if err == nil {
					s.logger.Info("scrapped block", slog.Int64("height", block.Height))
					s.SetBlock(block)
				} else if !reachedLatestHeight(fmt.Sprintf("%+v", err)) {
					// log only if it is not related to reached latest height error
					s.logger.Info("error while scrapping block", slog.Int64("height", h), slog.Any("error", err))
				}

				return nil
			})
		}

		if err := g.Wait(); err != nil {
			s.logger.Error("error while scrapping blocks", slog.Any("error", err))
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
