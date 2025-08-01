package move_nft

import (
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func getCollectionCreatorId(addr string, tx *gorm.DB) (int64, error) {
	addrBytes, err := util.HexToBytes(addr)
	if err != nil {
		return 0, err
	}

	var collection types.CollectedNftCollection
	if err := tx.
		Where("addr = ?", addrBytes).
		Limit(1).
		First(&collection).Error; err != nil {
		return 0, err
	}

	return collection.CreatorId, nil
}
