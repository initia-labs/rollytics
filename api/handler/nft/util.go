package nft

import "github.com/initia-labs/rollytics/types"

// getAccountIdMap is a generic function to get account information by IDs
func (h *NftHandler) getAccountIdMap(accountIds []int64) (map[int64][]byte, error) {
	var accounts []types.CollectedAccountDict
	if err := h.GetDatabase().
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

func (h *NftHandler) getCollectionCreatorIdMap(collections []types.CollectedNftCollection) (map[int64][]byte, error) {
	creatorIds := make([]int64, 0)
	for _, col := range collections {
		creatorIds = append(creatorIds, col.CreatorId)
	}
	return h.getAccountIdMap(creatorIds)
}

func (h *NftHandler) getNftOwnerIdMap(nfts []types.CollectedNft) (map[int64][]byte, error) {
	ownerIds := make([]int64, 0)
	for _, nft := range nfts {
		ownerIds = append(ownerIds, nft.OwnerId)
	}
	return h.getAccountIdMap(ownerIds)
}
