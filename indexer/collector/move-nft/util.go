package move_nft

import (
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

func getCollectionCreator(addr string, tx *gorm.DB) (string, error) {
	var collection types.CollectedNftCollection
	if err := tx.
		Where("addr = ?", addr).
		Limit(1).
		First(&collection).Error; err != nil {
		return "", err
	}

	return collection.Creator, nil
}
