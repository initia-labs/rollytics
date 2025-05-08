package nft

import (
	"github.com/initia-labs/rollytics/indexer/types"
	"gorm.io/gorm"
)

func (sub NftSubmodule) collect(block types.ScrappedBlock, tx *gorm.DB) (err error) {
	return nil
}
