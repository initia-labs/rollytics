package tx

import "github.com/initia-labs/rollytics/types"

func (h *TxHandler) getAccounts(txs []types.CollectedEvmInternalTx) (map[int64][]byte, error) {
	accountIds := make([]int64, 0)
	for _, tx := range txs {
		accountIds = append(accountIds, tx.FromId, tx.ToId)
	}

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

func (h *TxHandler) getHashs(txs []types.CollectedEvmInternalTx) (map[int64][]byte, error) {
	hashIds := make([]int64, 0)
	for _, tx := range txs {
		hashIds = append(hashIds, tx.HashId)
	}

	var hashs []types.CollectedEvmTxHashDict
	if err := h.GetDatabase().
		Where("id IN ?", hashIds).
		Find(&hashs).Error; err != nil {
		return nil, err
	}

	result := make(map[int64][]byte)
	for _, hash := range hashs {
		result[hash.Id] = hash.Hash
	}
	return result, nil
}
