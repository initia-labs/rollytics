package evmrichlist

import (
	"context"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

// GetBlockCollectedEvmTxs retrieves all evm transactions for a specific block height.
// Returns transactions ordered by sequence in ascending order.
func GetBlockCollectedEvmTxs(ctx context.Context, db *gorm.DB, height int64) ([]types.CollectedEvmTx, error) {
	var evmTxs []types.CollectedEvmTx

	if err := db.WithContext(ctx).
		Model(types.CollectedEvmTx{}).Where("height = ?", height).
		Order("sequence ASC").Find(&evmTxs).Error; err != nil {
		return nil, err
	}

	return evmTxs, nil
}

func GetBlockCollectedCosmosTxs(ctx context.Context, db *gorm.DB, height int64) ([]types.CollectedTx, error) {
	var cosmosTxs []types.CollectedTx

	if err := db.WithContext(ctx).
		Model(types.CollectedTx{}).Where("height = ?", height).
		Order("sequence ASC").Find(&cosmosTxs).Error; err != nil {
		return nil, err
	}

	return cosmosTxs, nil
}
