package tx

import (
	"github.com/gofiber/fiber/v2"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/types"
)

func (sub *TxSubmodule) prepare(block indexertypes.ScrapedBlock) (err error) {
	if sub.cfg.GetVmType() != types.EVM {
		return nil
	}

	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	evmTxs, err := getEvmTxs(client, sub.cfg, block.Height)
	if err != nil {
		return err
	}

	sub.mtx.Lock()
	sub.evmTxMap[block.Height] = evmTxs
	sub.mtx.Unlock()

	return nil
}
