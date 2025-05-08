package nft

import (
	"log/slog"

	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/types"
	"gorm.io/gorm"
)

const SubmoduleName = "nft"

var _ types.Submodule = NftSubmodule{}

type NftSubmodule struct {
	logger      *slog.Logger
	chainConfig *config.ChainConfig
	dataMap     map[int64]string
}

func New(logger *slog.Logger, chainConfig *config.ChainConfig) *NftSubmodule {
	return &NftSubmodule{
		logger:      logger.With("submodule", SubmoduleName),
		chainConfig: chainConfig,
		dataMap:     make(map[int64]string),
	}
}

func (sub NftSubmodule) Name() string {
	return SubmoduleName
}

func (sub NftSubmodule) Prepare(block types.ScrappedBlock) error {
	return sub.prepare(block)
}

func (sub NftSubmodule) Collect(block types.ScrappedBlock, tx *gorm.DB) error {
	return sub.collect(block, tx)
}
