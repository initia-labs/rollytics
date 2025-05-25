package nft

import (
	"errors"

	"github.com/initia-labs/rollytics/indexer/collector/nft/evm"
	"github.com/initia-labs/rollytics/indexer/collector/nft/move"
	"github.com/initia-labs/rollytics/indexer/collector/nft/wasm"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func (sub NftSubmodule) collect(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	data, dataOk := sub.dataMap[block.Height]
	defer delete(sub.dataMap, block.Height)

	switch sub.cfg.GetChainConfig().VmType {
	case types.MoveVM:
		if !dataOk {
			return errors.New("data is not prepared")
		}
		return move.Collect(block, data, sub.cfg, tx)
	case types.WasmVM:
		return wasm.Collect(block, sub.cfg, tx)
	case types.EVM:
		if !dataOk {
			return errors.New("data is not prepared")
		}
		return evm.Collect(block, data, sub.cfg, tx)
	default:
		return errors.New("invalid vm type")
	}
}
