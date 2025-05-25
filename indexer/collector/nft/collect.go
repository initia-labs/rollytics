package nft

import (
	"errors"

	"github.com/initia-labs/rollytics/indexer/collector/nft/evm"
	"github.com/initia-labs/rollytics/indexer/collector/nft/move"
	"github.com/initia-labs/rollytics/indexer/collector/nft/pair"
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
		if err = move.Collect(block, data, sub.cfg, tx); err != nil {
			return err
		}
	case types.WasmVM:
		if err = wasm.Collect(block, sub.cfg, tx); err != nil {
			return err
		}
	case types.EVM:
		if !dataOk {
			return errors.New("data is not prepared")
		}
		if err = evm.Collect(block, data, sub.cfg, tx); err != nil {
			return err
		}
	default:
		return errors.New("invalid vm type")
	}

	return pair.Collect(block, sub.cfg, tx)
}
