package tx

import (
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
)

func buildTxEdgeQuery(tx *gorm.DB, accountID int64, isSigner bool, msgTypeIds []int64, pagination *common.Pagination) (*gorm.DB, int64, error) {
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

	total, err := common.GetCountWithTimeout(countQuery, pagination.CountTotal)
	if err != nil {
		return nil, 0, err
	}

	// apply pagination to the sequence query
	sequenceQuery = pagination.ApplySequence(sequenceQuery)

	query := tx.Model(&types.CollectedTx{}).
		Where("sequence IN (?)", sequenceQuery).
		Order(pagination.OrderBy("sequence"))

	return query, total, nil
}

func buildEdgeQueryForGetTxs(tx *gorm.DB, msgTypeIds []int64, pagination *common.Pagination) (*gorm.DB, int64, error) {
	sequenceQuery := tx.
		Model(&types.CollectedTxMsgType{}).
		Select("sequence")

	if len(msgTypeIds) > 0 {
		sequenceQuery = sequenceQuery.Where("msg_type_id = ANY(?)", pq.Array(msgTypeIds))
	}

	sequenceQuery = sequenceQuery.Distinct("sequence")
	countQuery := sequenceQuery.Session(&gorm.Session{})

	total, err := common.GetCountWithTimeout(countQuery, pagination.CountTotal)
	if err != nil {
		return nil, 0, err
	}

	// apply pagination to the sequence query
	sequenceQuery = pagination.ApplySequence(sequenceQuery)

	query := tx.Model(&types.CollectedTx{}).
		Where("sequence IN (?)", sequenceQuery).
		Order(pagination.OrderBy("sequence"))

	return query, total, nil
}

func buildEdgeQueryForGetTxsByHeight(tx *gorm.DB, height int64, msgTypeIds []int64, pagination *common.Pagination) (*gorm.DB, int64, error) {
	sequenceQuery := tx.
		Model(&types.CollectedTxMsgType{}).
		Select("sequence")

	if len(msgTypeIds) > 0 {
		sequenceQuery = sequenceQuery.Where("msg_type_id = ANY(?)", pq.Array(msgTypeIds))
	}

	sequenceQuery = sequenceQuery.Distinct("sequence")

	query := tx.Model(&types.CollectedTx{}).
		Where("height = ?", height).
		Where("sequence IN (?)", sequenceQuery)

	var strategy types.CollectedTx
	const hasFilters = true // always filtering by msg_type_ids and height

	total, err := common.GetOptimizedCount(query, strategy, hasFilters, pagination.CountTotal)
	if err != nil {
		return nil, 0, err
	}

	query = pagination.ApplySequence(query)

	return query, total, nil
}
