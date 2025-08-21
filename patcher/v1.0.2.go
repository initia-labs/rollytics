package patcher

import (
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func PatchV1_0_2(tx *gorm.DB, cfg *config.Config) error {
	switch cfg.GetVmType() {
	case types.EVM:
		return nil
	case types.WasmVM:
		// TODO

		return nil
	case types.MoveVM:
		// TODO
		return nil
	}

	return nil
}
