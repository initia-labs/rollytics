package common

import (
	"errors"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

// EdgeBackfillStatus captures whether an edge backfill has completed and the
// most recent sequence recorded for that backfill marker.
type EdgeBackfillStatus struct {
	Completed bool
	Sequence  int64
}

// GetEdgeBackfillStatus retrieves backfill information for the provided
// sequence name.
func GetEdgeBackfillStatus(tx *gorm.DB, name types.SeqInfoName) (EdgeBackfillStatus, error) {
	var seqInfo types.CollectedSeqInfo
	if err := tx.
		Model(&types.CollectedSeqInfo{}).
		Where("name = ?", name).
		First(&seqInfo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return EdgeBackfillStatus{}, nil
		}
		return EdgeBackfillStatus{}, err
	}

	return EdgeBackfillStatus{
		Completed: seqInfo.Sequence == -1,
		Sequence:  seqInfo.Sequence,
	}, nil
}

// IsEdgeBackfillReady is a helper that retains the prior boolean interface for
// callers that only care about completion.
func IsEdgeBackfillReady(tx *gorm.DB, name types.SeqInfoName) (bool, error) {
	status, err := GetEdgeBackfillStatus(tx, name)
	if err != nil {
		return false, err
	}
	return status.Completed, nil
}
