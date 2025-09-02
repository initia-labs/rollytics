package tx

import (
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func (h *TxHandler) getAccounts(tx *gorm.DB, txs []types.CollectedEvmInternalTx) (map[int64][]byte, error) {
	accountIds := make([]int64, 0)
	for _, t := range txs {
		accountIds = append(accountIds, t.FromId, t.ToId)
	}

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

func (h *TxHandler) getHashes(tx *gorm.DB, txs []types.CollectedEvmInternalTx) (map[int64][]byte, error) {
	hashIds := make([]int64, 0)
	for _, t := range txs {
		hashIds = append(hashIds, t.HashId)
	}

	var hashes []types.CollectedEvmTxHashDict
	if err := tx.
		Where("id IN ?", hashIds).
		Find(&hashes).Error; err != nil {
		return nil, err
	}

	result := make(map[int64][]byte)
	for _, hash := range hashes {
		result[hash.Id] = hash.Hash
	}
	return result, nil
}
