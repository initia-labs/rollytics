package nft

import (
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func (h *NftHandler) getAccountIdMap(tx *gorm.DB, accountIds []int64) (map[int64][]byte, error) {
	var accounts []types.CollectedAccountDict
	if err := tx.
		Where("id IN ?", accountIds).
		Find(&accounts).Error; err != nil {
		return nil, err
	}

	result := make(map[int64][]byte)
	for _, acc := range accounts {
		result[acc.Id] = acc.Account
	}
	return result, nil
}

func (h *NftHandler) getCollectionCreatorIdMap(tx *gorm.DB, collections []types.CollectedNftCollection) (map[int64][]byte, error) {
	creatorIds := make([]int64, 0, len(collections))
	for _, col := range collections {
		creatorIds = append(creatorIds, col.CreatorId)
	}
	return h.getAccountIdMap(tx, creatorIds)
}

func (h *NftHandler) getNftOwnerIdMap(tx *gorm.DB, nfts []types.CollectedNft) (map[int64][]byte, error) {
	ownerIds := make([]int64, 0, len(nfts))
	for _, nft := range nfts {
		ownerIds = append(ownerIds, nft.OwnerId)
	}
	return h.getAccountIdMap(tx, ownerIds)
}
