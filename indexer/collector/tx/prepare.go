package tx

import (
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"

	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/types"
)

func (sub *TxSubmodule) prepare(block indexertypes.ScrapedBlock) error {
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	var g errgroup.Group
	var restTxs []RestTx
	var evmTxs []types.EvmTx

	g.Go(func() error {
		txs, err := getRestTxs(client, sub.cfg, block.Height, len(block.Txs))
		if err != nil {
			return err
		}

		restTxs = txs
		return nil
	})

	g.Go(func() error {
		txs, err := getEvmTxs(client, sub.cfg, block.Height)
		if err != nil {
			return err
		}

		evmTxs = txs
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	sub.mtx.Lock()
	sub.cache[block.Height] = CacheData{
		RestTxs: restTxs,
		EvmTxs:  evmTxs,
	}
	sub.mtx.Unlock()

	return nil
}
