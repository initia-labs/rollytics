package nft

import (
	"errors"

	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func (sub NftSubmodule) collect(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	switch sub.cfg.GetChainConfig().VmType {
	case types.MoveVM:
		return sub.collectMove(block, tx)
	case types.WasmVM:
		return sub.collectWasm(block, tx)
	case types.EVM:
		return sub.collectEvm(block, tx)
	default:
		return errors.New("invalid vm type")
	}
}
