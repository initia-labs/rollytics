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
	blockMap map[int64]types.ScrappedBlock
	mtx      sync.Mutex
}

func New(cfg *config.Config, logger *slog.Logger) *Scrapper {
	return &Scrapper{
		cfg:      cfg,
		logger:   logger.With("module", "scrapper"),
		blockMap: make(map[int64]types.ScrappedBlock),
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

func (s *Scrapper) GetBlock(height int64) (types.ScrappedBlock, bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	block, ok := s.blockMap[height]
	return block, ok
}

func (s *Scrapper) SetBlock(block types.ScrappedBlock) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.blockMap[block.Height] = block
}

func (s *Scrapper) DeleteBlock(height int64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	delete(s.blockMap, height)
}

func (s *Scrapper) GetBlockMapSize() int {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return len(s.blockMap)
}
