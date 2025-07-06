package util

import (
	"errors"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

func GetSeqInfo(chainId string, name string, tx *gorm.DB) (seqInfo types.CollectedSeqInfo, err error) {
	if err := tx.Where("chain_id = ? AND name = ?", chainId, name).First(&seqInfo).Error; err != nil {
		// initialize if not found
		if errors.Is(err, gorm.ErrRecordNotFound) {
			seqInfo = types.CollectedSeqInfo{
				ChainId:  chainId,
				Name:     name,
				Sequence: 0,
			}
		} else {
			return seqInfo, err
		}
	}

	return seqInfo, nil
}
