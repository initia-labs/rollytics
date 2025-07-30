package util

import (
	"errors"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

func GetSeqInfo(name types.SeqInfoName, tx *gorm.DB) (seqInfo types.CollectedSeqInfo, err error) {
	if err := tx.Where("name = ?", name).First(&seqInfo).Error; err != nil {
		// initialize if not found
		if errors.Is(err, gorm.ErrRecordNotFound) {
			seqInfo = types.CollectedSeqInfo{
				Name:     string(name),
				Sequence: 0,
			}
		} else {
			return seqInfo, err
		}
	}

	return seqInfo, nil
}
