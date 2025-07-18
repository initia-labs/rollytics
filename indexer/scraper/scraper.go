package scraper

import (
	"log/slog"
	"sync"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/types"
)

const (
	layout         = "2006-01-02T15:04:05.999999999Z"
	batchScrapSize = 5
	maxErrCount    = 5
)

type Scraper struct {
	cfg    *config.Config
	logger *slog.Logger
	mtx    sync.Mutex
}

func New(cfg *config.Config, logger *slog.Logger) *Scraper {
	return &Scraper{
		cfg:    cfg,
		logger: logger.With("module", "scraper"),
	}
}

func (s *Scraper) Run(height int64, blockChan chan<- types.ScrapedBlock, controlChan <-chan string) {
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	s.logger.Info("fast syncing until fully synced")
	syncedHeight := s.fastSync(client, height, blockChan, controlChan)

	s.logger.Info("switching to slow syncing")
	s.slowSync(client, syncedHeight+1, blockChan)
}
