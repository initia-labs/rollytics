package move_nft

import (
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

func getCollectionCreator(chainId, addr string, tx *gorm.DB) (string, error) {
	var collection types.CollectedNftCollection
	if res := tx.Where("chain_id = ? AND addr = ?", chainId, addr).Limit(1).Take(&collection); res.Error != nil {
		return "", res.Error
	}

	return collection.Creator, nil
}
