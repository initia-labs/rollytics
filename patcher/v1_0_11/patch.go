package v1_0_11

import (
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
)

type TxInfo struct {
	Height    int64
	Sequence  int64
	Timestamp time.Time
}

type TransferInfo struct {
	Owner  string
	TxInfo TxInfo
}

func Patch(tx *gorm.DB, cfg *config.Config, logger *slog.Logger) error {
	switch cfg.GetVmType() {
	case types.EVM:
		return nil
	case types.WasmVM:
		return PatchWasmNFT(tx, cfg, logger)
	case types.MoveVM:
		return nil
	}

	return nil
}
