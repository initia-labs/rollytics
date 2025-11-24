package tx

import (
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/common-handler/common"
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

func buildSequenceQueryWithMsgTypeFilter(tx *gorm.DB, msgTypeIds []int64) *gorm.DB {
	query := tx.Model(&types.CollectedTxMsgType{})

	if len(msgTypeIds) > 0 {
		query = query.Where("msg_type_id = ANY(?)", pq.Array(msgTypeIds))
	}

	return query.Distinct("sequence")
}

func buildSequenceQueryWithHeightAndMsgTypeFilter(tx *gorm.DB, height int64, msgTypeIds []int64) *gorm.DB {
	txTable := types.CollectedTx{}.TableName()
	mttTable := types.CollectedTxMsgType{}.TableName()

	// Start from CollectedTx filtered by height (uses height index, gets small subset first)
	// This is optimal because height is highly selective (one height = few transactions)
	query := tx.Model(&types.CollectedTx{}).
		Select(txTable+".sequence").
		Where(txTable+".height = ?", height)

	// If msg_type filter is needed, join to CollectedTxMsgType
	if len(msgTypeIds) > 0 {
		query = query.
			Joins("JOIN "+mttTable+" ON "+mttTable+".sequence = "+txTable+".sequence").
			Where(mttTable+".msg_type_id = ANY(?)", pq.Array(msgTypeIds))
	}

	return query.Distinct(txTable + ".sequence")
}

func buildEdgeQueryForGetTxs(tx *gorm.DB, msgTypeIds []int64, pagination *common.Pagination) (*gorm.DB, int64, error) {
	sequenceQuery := buildSequenceQueryWithMsgTypeFilter(tx, msgTypeIds)

	hasFilters := len(msgTypeIds) > 0

	var total int64
	var err error
	if !hasFilters && pagination.CountTotal {
		total, err = common.GetOptimizedCount(
			tx.Model(&types.CollectedTxMsgType{}),
			types.CollectedTxMsgType{},
			false,
			pagination.CountTotal,
		)
	} else {
		countQuery := buildSequenceQueryWithMsgTypeFilter(tx, msgTypeIds)
		total, err = common.GetCountWithTimeout(countQuery, pagination.CountTotal)
	}

	if err != nil {
		return nil, 0, err
	}

	sequenceQuery = pagination.ApplySequence(sequenceQuery)

	query := tx.Model(&types.CollectedTx{}).
		Where("sequence IN (?)", sequenceQuery).
		Order(pagination.OrderBy("sequence"))

	return query, total, nil
}

func buildEdgeQueryForGetTxsByHeight(tx *gorm.DB, height int64, msgTypeIds []int64, pagination *common.Pagination) (*gorm.DB, int64, error) {
	sequenceQuery := buildSequenceQueryWithHeightAndMsgTypeFilter(tx, height, msgTypeIds)

	hasFilters := len(msgTypeIds) > 0

	var total int64
	var err error
	if !hasFilters && pagination.CountTotal {
		// Optimize: when no msg_type filter, count directly from CollectedTx by height
		total, err = common.GetOptimizedCount(
			tx.Model(&types.CollectedTx{}).Where("height = ?", height),
			types.CollectedTx{},
			true, // hasFilters (height filter)
			pagination.CountTotal,
		)
	} else {
		countQuery := buildSequenceQueryWithHeightAndMsgTypeFilter(tx, height, msgTypeIds)
		total, err = common.GetCountWithTimeout(countQuery, pagination.CountTotal)
	}

	if err != nil {
		return nil, 0, err
	}

	sequenceQuery = pagination.ApplySequence(sequenceQuery)

	query := tx.Model(&types.CollectedTx{}).
		Where("sequence IN (?)", sequenceQuery).
		Order(pagination.OrderBy("sequence"))

	return query, total, nil
}
