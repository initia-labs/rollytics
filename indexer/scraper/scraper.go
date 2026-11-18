package scraper

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/metrics"
	commontypes "github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/querier"
)

const (
	layout = "2006-01-02T15:04:05.999999999Z"
)

type Scraper struct {
	cfg            *config.Config
	querier        *querier.Querier
	logger         *slog.Logger
	mtx            sync.Mutex
	lastScrapeTime time.Time
	scrapedCount   int64
}

func New(cfg *config.Config, logger *slog.Logger) *Scraper {
	return &Scraper{
		cfg:            cfg,
		querier:        querier.NewQuerier(cfg.GetChainConfig()),
		logger:         logger.With("module", "scraper"),
		lastScrapeTime: time.Now(),
		scrapedCount:   0,
	}
}

func (s *Scraper) Run(ctx context.Context, height int64, blockChan chan<- types.ScrapedBlock, controlChan <-chan string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	var paused atomic.Bool
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case signal, ok := <-controlChan:
				if !ok {
					return
				}
				switch signal {
				case "pause":
					paused.Store(true)
				case "start":
					paused.Store(false)
				}
			}
		}
	}()

	// Start metrics updater
	go s.updateScrapeSpeedMetrics(ctx)

	s.logger.Info("fast syncing until fully synced")
	syncedHeight := s.fastSync(ctx, client, height, blockChan, &paused)

	s.logger.Info("switching to slow syncing")
	s.slowSync(ctx, client, syncedHeight+1, blockChan, &paused)
}

// updateScrapeSpeedMetrics periodically updates scrape speed metrics
func (s *Scraper) updateScrapeSpeedMetrics(ctx context.Context) {
	ticker := time.NewTicker(commontypes.ScrapeSpeedUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		s.mtx.Lock()
		now := time.Now()
		elapsed := now.Sub(s.lastScrapeTime).Seconds()
		if elapsed > 0 {
			speed := float64(s.scrapedCount) / elapsed
			metrics.GetMetrics().IndexerMetrics().ProcessingSpeed.Set(speed)
		}
		s.scrapedCount = 0
		s.lastScrapeTime = now
		s.mtx.Unlock()
	}
}

// trackScrapedBlock increments the scraped block counter
func (s *Scraper) trackScrapedBlock() {
	s.mtx.Lock()
	s.scrapedCount++
	s.mtx.Unlock()
}
