package v1_0_2

import (
	"time"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	movenft "github.com/initia-labs/rollytics/indexer/collector/move-nft"
	"github.com/initia-labs/rollytics/types"
)

type TxInfo struct {
	Height    int64
	Sequence  int64
	Timestamp time.Time
}

type CollectionEventInfo struct {
	Event  movenft.CreateCollectionEvent
	TxInfo TxInfo
}

type TransferInfo struct {
	Owner  string
	TxInfo TxInfo
}

type MutationInfo struct {
	Uri    string
	TxInfo TxInfo
}

func Patch(tx *gorm.DB, cfg *config.Config) error {
	switch cfg.GetVmType() {
	case types.EVM:
		return nil
	case types.WasmVM:
		return PatchWasmNFT(tx, cfg)
	case types.MoveVM:
		return PatchMoveNFT(tx, cfg)
	}

	return nil
}
