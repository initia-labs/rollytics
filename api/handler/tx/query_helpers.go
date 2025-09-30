package tx

import (
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
)

func buildEdgeQueryForGetTxs(tx *gorm.DB, msgTypeIds []int64) (*gorm.DB, int64, error) {
	sequenceQuery := tx.
		Model(&types.CollectedTxMsgType{}).
		Select("sequence")

	if len(msgTypeIds) > 0 {
		sequenceQuery = sequenceQuery.Where("msg_type_id = ANY(?)", pq.Array(msgTypeIds))
	}

	sequenceQuery = sequenceQuery.Distinct("sequence")
	countQuery := sequenceQuery.Session(&gorm.Session{})

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := tx.Model(&types.CollectedTx{}).
		Where("sequence IN (?)", sequenceQuery)

	return query, total, nil
}

func buildEdgeQueryForGetTxsByHeight(tx *gorm.DB, height int64, msgTypeIds []int64) (*gorm.DB, int64, error) {
	sequenceQuery := tx.
		Model(&types.CollectedTxMsgType{}).
		Select("sequence")

	if len(msgTypeIds) > 0 {
		sequenceQuery = sequenceQuery.Where("msg_type_id = ANY(?)", pq.Array(msgTypeIds))
	}

	sequenceQuery = sequenceQuery.Distinct("sequence")

	baseQuery := tx.Model(&types.CollectedTx{}).
		Where("height = ?", height).
		Where("sequence IN (?)", sequenceQuery)

	var total int64
	if err := baseQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	return baseQuery, total, nil
}

func buildTxEdgeQuery(tx *gorm.DB, accountID int64, isSigner bool, msgTypeIds []int64) (*gorm.DB, int64, error) {
	sequenceQuery := tx.
		Model(&types.CollectedTxAccount{}).
		Select("sequence").
		Where("account_id = ?", accountID)

	if isSigner {
		sequenceQuery = sequenceQuery.Where("signer")
	}

	if len(msgTypeIds) > 0 {
		at := types.CollectedTxAccount{}.TableName()
		mtt := types.CollectedTxMsgType{}.TableName()
		sequenceQuery = sequenceQuery.
			Joins("JOIN "+mtt+" ON "+mtt+".sequence = "+at+".sequence").
			Where(mtt+".msg_type_id = ANY(?)", pq.Array(msgTypeIds))
	}

	sequenceQuery = sequenceQuery.Distinct("sequence")
	countQuery := sequenceQuery.Session(&gorm.Session{})

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := tx.Model(&types.CollectedTx{}).
		Where("sequence IN (?)", sequenceQuery)

	return query, total, nil
}

func buildTxLegacyQuery(tx *gorm.DB, accountIds []int64, isSigner bool, msgTypeIds []int64) (*gorm.DB, int64, error) {
	query := tx.Model(&types.CollectedTx{}).
		Where("account_ids && ?", pq.Array(accountIds))

	if isSigner {
		query = query.Where("signer_id = ?", accountIds[0])
	}

	if len(msgTypeIds) > 0 {
		query = query.Where("msg_type_ids && ?", pq.Array(msgTypeIds))
	}

	var strategy types.CollectedTx
	const hasFilters = true // always filtering by account_ids

	total, err := common.GetOptimizedCount(query, strategy, hasFilters)
	if err != nil {
		return nil, 0, err
	}

	return query, total, nil
}
