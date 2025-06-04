package tx

import (
	"errors"

	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func getSeqInfo(chainId string, name string, tx *gorm.DB) (seqInfo types.CollectedSeqInfo, err error) {
	if res := tx.Where("chain_id = ? AND name = ?", chainId, name).Take(&seqInfo); res.Error != nil {
		// initialize if not found
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			seqInfo = types.CollectedSeqInfo{
				ChainId:  chainId,
				Name:     name,
				Sequence: 0,
			}
		} else {
			return seqInfo, res.Error
		}
	}

	return seqInfo, nil
}
