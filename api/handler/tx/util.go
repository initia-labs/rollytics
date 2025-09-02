package tx

import (
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

func (h *TxHandler) getAccounts(tx *gorm.DB, txs []types.CollectedEvmInternalTx) (map[int64][]byte, error) {
	// Early return for empty input
	if len(txs) == 0 {
		return make(map[int64][]byte), nil
	}

	// Use map to deduplicate account IDs, filtering out invalid IDs (≤ 0)
	accountIdSet := make(map[int64]struct{})
	for _, t := range txs {
		if t.FromId > 0 {
			accountIdSet[t.FromId] = struct{}{}
		}
		if t.ToId > 0 {
			accountIdSet[t.ToId] = struct{}{}
		}
	}

	// Early return if no valid account IDs found
	if len(accountIdSet) == 0 {
		return make(map[int64][]byte), nil
	}

	// Convert map keys to slice
	accountIds := make([]int64, 0, len(accountIdSet))
	for id := range accountIdSet {
		accountIds = append(accountIds, id)
	}

	var accounts []types.CollectedAccountDict
	if err := tx.
		Where("id IN ?", accountIds).
		Find(&accounts).Error; err != nil {
		return nil, err
	}

	result := make(map[int64][]byte, len(accounts))
	for _, acc := range accounts {
		result[acc.Id] = acc.Account
	}
	return result, nil
}

func (h *TxHandler) getHashes(tx *gorm.DB, txs []types.CollectedEvmInternalTx) (map[int64][]byte, error) {
	// Early return for empty input
	if len(txs) == 0 {
		return make(map[int64][]byte), nil
	}

	// Use map to deduplicate hash IDs, filtering out invalid IDs (≤ 0)
	hashIdSet := make(map[int64]struct{})
	for _, t := range txs {
		if t.HashId > 0 {
			hashIdSet[t.HashId] = struct{}{}
		}
	}

	// Early return if no valid hash IDs found
	if len(hashIdSet) == 0 {
		return make(map[int64][]byte), nil
	}

	// Convert map keys to slice
	hashIds := make([]int64, 0, len(hashIdSet))
	for id := range hashIdSet {
		hashIds = append(hashIds, id)
	}

	var hashes []types.CollectedEvmTxHashDict
	if err := tx.
		Where("id IN ?", hashIds).
		Find(&hashes).Error; err != nil {
		return nil, err
	}

	result := make(map[int64][]byte, len(hashes))
	for _, hash := range hashes {
		result[hash.Id] = hash.Hash
	}
	return result, nil
}
