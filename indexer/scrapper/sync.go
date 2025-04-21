package scrapper

import (
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

func (s *Scrapper) fastSync(client *fiber.Client, height int64) (syncedHeight int64) {
	for !s.synced {
		if len(s.BlockMap) > 100 {
			time.Sleep(1 * time.Second)
			continue
		}

		// spin up new goroutine for scrapping block with incrementing height
		go func(h int64, errCount int) {
			for {
				block, err := scrapBlock(client, h, s.cfg)

				// if no error, cache the scrapped block to block map and return
				if err == nil {
					s.logger.Info("scrapped block", slog.Int64("height", block.Height))
					s.BlockMap[block.Height] = block

					s.mtx.Lock()
					if block.Height > syncedHeight {
						syncedHeight = block.Height
					}
					s.mtx.Unlock()
					return
				}

				// stop fast syncing after reaching latest height
				if reachedLatestHeight(fmt.Sprintf("%+v", err)) {
					s.mtx.Lock()
					s.synced = true
					s.mtx.Unlock()
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
		}(height, 0)

		height++
		time.Sleep(s.cfg.GetCoolingDuration())
	}

	return
}

func (s *Scrapper) slowSync(client *fiber.Client, height int64) {
	for {
		var results []ScrapResult
		var g errgroup.Group

		for i := range batchScrapSize {
			g.Go(func(h int64) func() error {
				return func() error {
					block, err := scrapBlock(client, h, s.cfg)
					s.mtx.Lock()
					results = append(results, ScrapResult{
						Height: h,
						Err:    err,
					})
					s.mtx.Unlock()

					if err == nil {
						s.logger.Info("scrapped block", slog.Int64("height", block.Height))
						s.BlockMap[block.Height] = block
					} else if !reachedLatestHeight(fmt.Sprintf("%+v", err)) {
						// log only if it is not related to reached latest height error
						s.logger.Info("error while scrapping block", slog.Int64("height", h), slog.Any("error", err))
					}

					return nil
				}
			}(height + int64(i)))
		}

		if err := g.Wait(); err != nil {
			s.logger.Error("error while scrapping blocks", slog.Any("error", err))
			time.Sleep(s.cfg.GetCoolingDuration())
			continue
		}

		sort.Slice(results, func(i, j int) bool {
			return results[i].Height < results[j].Height
		})

		s.mtx.Lock()
		lowestHeightWithError := int64(0)
		allSucceeded := true
		for _, res := range results {
			if res.Err != nil {
				allSucceeded = false
				lowestHeightWithError = res.Height
				break
			}
		}

		if allSucceeded {
			height += batchScrapSize
		} else {
			height = lowestHeightWithError
		}

		s.mtx.Unlock()
		time.Sleep(s.cfg.GetCoolingDuration())
	}
}
