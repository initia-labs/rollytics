package scraper

import (
	"context"
	"log/slog"
	"sync"
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
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	// Start metrics updater
	go s.updateScrapeSpeedMetrics()

	s.logger.Info("fast syncing until fully synced")
	syncedHeight := s.fastSync(ctx, client, height, blockChan, controlChan)

	s.logger.Info("switching to slow syncing")
	s.slowSync(ctx, client, syncedHeight+1, blockChan)
}

// updateScrapeSpeedMetrics periodically updates scrape speed metrics
func (s *Scraper) updateScrapeSpeedMetrics() {
	ticker := time.NewTicker(commontypes.ScrapeSpeedUpdateInterval)
	defer ticker.Stop()

	for range ticker.C {
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
