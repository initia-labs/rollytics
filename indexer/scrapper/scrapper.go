package scrapper

import (
	"log/slog"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/types"
)

const (
	layout         = "2006-01-02T15:04:05.999999999Z"
	batchScrapSize = 5
	maxErrCount    = 5
)

type Scrapper struct {
	cfg      *config.Config
	logger   *slog.Logger
	synced   bool
	BlockMap map[int64]types.ScrappedBlock
	mtx      sync.Mutex
}

func New(cfg *config.Config, logger *slog.Logger) *Scrapper {
	return &Scrapper{
		cfg:      cfg,
		logger:   logger.With("module", "scrapper"),
		synced:   false,
		BlockMap: make(map[int64]types.ScrappedBlock),
	}
}

func (s *Scrapper) Run(height int64) {
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	s.logger.Info("fast syncing until fully synced")
	syncedHeight := s.fastSync(client, height)

	s.logger.Info("switching to slow syncing")
	s.slowSync(client, syncedHeight+1)
}

func (s *Scrapper) DeleteBlock(height int64) {
	s.mtx.Lock()
	delete(s.BlockMap, height)
	s.mtx.Unlock()
}
