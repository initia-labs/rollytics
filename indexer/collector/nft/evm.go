package nft

import (
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"gorm.io/gorm"
)

func (sub NftSubmodule) prepareEvm(block indexertypes.ScrappedBlock) (err error) {
	return nil
}

func (sub NftSubmodule) collectEvm(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	return nil
}
