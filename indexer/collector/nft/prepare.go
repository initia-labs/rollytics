package nft

import (
	"errors"

	"github.com/initia-labs/rollytics/indexer/collector/nft/evm"
	"github.com/initia-labs/rollytics/indexer/collector/nft/move"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/types"
)

func (sub *NftSubmodule) prepare(block indexertypes.ScrappedBlock) (err error) {
	switch sub.cfg.GetChainConfig().VmType {
	case types.MoveVM:
		data, err := move.Prepare(block, sub.cfg)
		if err != nil {
			return err
		}
		sub.mtx.Lock()
		sub.dataMap[block.Height] = data
		sub.mtx.Unlock()
		return nil
	case types.WasmVM:
		return nil
	case types.EVM:
		data, err := evm.Prepare(block, sub.cfg)
		if err != nil {
			return err
		}
		sub.mtx.Lock()
		sub.dataMap[block.Height] = data
		sub.mtx.Unlock()
		return nil
	default:
		return errors.New("invalid vm type")
	}
}
