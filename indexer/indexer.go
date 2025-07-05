package indexer

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/collector"
	"github.com/initia-labs/rollytics/indexer/scraper"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

type Indexer struct {
	cfg          *config.Config
	logger       *slog.Logger
	db           *orm.Database
	scraper      *scraper.Scraper
	collector    *collector.Collector
	blockMap     map[int64]indexertypes.ScrapedBlock
	blockChan    chan indexertypes.ScrapedBlock
	controlChan  chan string
	paused       bool
	mtx          sync.Mutex
	height       int64
	prepareCount int
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *Indexer {
	return &Indexer{
		cfg:         cfg,
		logger:      logger,
		db:          db,
		scraper:     scraper.New(cfg, logger),
		collector:   collector.New(cfg, logger, db),
		blockMap:    make(map[int64]indexertypes.ScrapedBlock),
		blockChan:   make(chan indexertypes.ScrapedBlock),
		controlChan: make(chan string),
	}
}

func (i *Indexer) Run() error {
	var lastBlock types.CollectedBlock
	res := i.db.Where("chain_id = ?", i.cfg.GetChainId()).Order("height desc").Limit(1).Take(&lastBlock)
	if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		i.logger.Error("failed to get the last block from db", slog.Any("error", res.Error))
		return errors.New("failed to get the last block from db")
	}
	i.height = lastBlock.Height + 1

	go i.scrape()
	go i.prepare()
	i.collect()

	return nil
}

func (i *Indexer) scrape() {
	i.scraper.Run(i.height, i.blockChan, i.controlChan)
}

func (i *Indexer) prepare() {
	for block := range i.blockChan {
		if block.Height < i.height {
			continue
		}

		i.mtx.Lock()
		i.prepareCount++
		i.mtx.Unlock()

		b := block
		go func() {
			if err := i.collector.Prepare(b); err != nil {
				panic(err)
			}

			i.mtx.Lock()
			i.blockMap[b.Height] = b
			i.prepareCount--
			i.mtx.Unlock()
		}()
	}
}

func (i *Indexer) collect() {
	for {
		i.mtx.Lock()

		switch {
		case (len(i.blockMap) > 100 || i.prepareCount > 100) && !i.paused:
			i.controlChan <- "stop"
			i.paused = true
		case len(i.blockMap) < 50 && i.prepareCount < 50 && i.paused:
			i.controlChan <- "start"
			i.paused = false
		}

		block, ok := i.blockMap[i.height]
		delete(i.blockMap, i.height)
		i.mtx.Unlock()

		if !ok {
			time.Sleep(i.cfg.GetCoolingDuration())
			continue
		}

		if err := i.collector.Collect(block); err != nil {
			panic(err)
		}

		i.height++
	}
}
