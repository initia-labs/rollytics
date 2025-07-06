package move_nft

import (
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

func getCollectionCreator(chainId, addr string, tx *gorm.DB) (string, error) {
	var collection types.CollectedNftCollection
	if err := tx.
		Where("chain_id = ? AND addr = ?", chainId, addr).
		Limit(1).
		First(&collection).Error; err != nil {
		return "", err
	}

	return collection.Creator, nil
}
