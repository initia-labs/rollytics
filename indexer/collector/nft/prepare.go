package nft

import (
	"errors"

	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/types"
)

func (sub NftSubmodule) prepare(block indexertypes.ScrappedBlock) (err error) {
	switch sub.chainConfig.VmType {
	case types.MoveVM:
		return sub.prepareMove(block)
	case types.WasmVM:
		return sub.prepareWasm(block)
	case types.EVM:
		return sub.prepareEvm(block)
	default:
		return errors.New("invalid vm type")
	}
}

func (sub NftSubmodule) prepareMove(block indexertypes.ScrappedBlock) (err error) {
	return nil
}

func (sub NftSubmodule) prepareWasm(block indexertypes.ScrappedBlock) (err error) {
	return nil
}

func (sub NftSubmodule) prepareEvm(block indexertypes.ScrappedBlock) (err error) {
	return nil
}
