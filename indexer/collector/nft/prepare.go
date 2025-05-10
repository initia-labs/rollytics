package nft

import (
	"errors"

	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/types"
)

func (sub NftSubmodule) prepare(block indexertypes.ScrappedBlock) (err error) {
	switch sub.cfg.GetChainConfig().VmType {
	case types.MoveVM:
		return sub.prepareMove(block)
	case types.WasmVM:
		return nil
	case types.EVM:
		return sub.prepareEvm(block)
	default:
		return errors.New("invalid vm type")
	}
}
