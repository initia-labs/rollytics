package tx

import (
	"context"

	"golang.org/x/sync/errgroup"

	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/types"
)

func (sub *TxSubmodule) prepare(ctx context.Context, block indexertypes.ScrapedBlock) error {
	var g errgroup.Group
	var restTxs []types.RestTx
	var evmTxs []types.EvmTx

	g.Go(func() error {
		txs, err := sub.querier.GetCosmosTxs(ctx, block.Height, len(block.Txs))
		if err != nil {
			return err
		}

		restTxs = txs
		return nil
	})

	g.Go(func() error {
		txs, err := sub.querier.GetEvmTxs(ctx, block.Height)
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
