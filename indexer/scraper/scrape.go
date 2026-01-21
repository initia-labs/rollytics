package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	cbjson "github.com/cometbft/cometbft/libs/json"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/metrics"
	"github.com/initia-labs/rollytics/util/querier"
)

func scrapeBlock(ctx context.Context, client *fiber.Client, height int64, cfg *config.Config, q *querier.Querier) (types.ScrapedBlock, error) {
	start := time.Now()

	var g errgroup.Group
	getBlockRes := make(chan GetBlockResponse, 1)
	getBlockResultsRes := make(chan GetBlockResultsResponse, 1)

	g.Go(func() error {
		defer close(getBlockRes)
		return fetchBlock(ctx, client, height, cfg, q.RpcUrls, getBlockRes)
	})

	g.Go(func() error {
		defer close(getBlockResultsRes)
		return fetchBlockResults(ctx, client, height, cfg, q.RpcUrls, getBlockResultsRes)
	})

	indexerMetrics := metrics.GetMetrics().IndexerMetrics()

	if err := g.Wait(); err != nil {
		indexerMetrics.ProcessingErrors.WithLabelValues("scrape", "network_error").Inc()
		return types.ScrapedBlock{}, err
	}

	block := <-getBlockRes
	blockResults := <-getBlockResultsRes

	scrapedBlock, err := parseScrapedBlock(block, blockResults, height)
	if err != nil {
		indexerMetrics.ProcessingErrors.WithLabelValues("scrape", "parse_error").Inc()
		return types.ScrapedBlock{}, err
	}

	// Track scrape metrics
	duration := time.Since(start).Seconds()
	indexerMetrics.BlockProcessingTime.WithLabelValues("scrape").Observe(duration)

	return scrapedBlock, nil
}

func fetchBlock(ctx context.Context, client *fiber.Client, height int64, cfg *config.Config, rpcURLs []string, getBlockRes chan<- GetBlockResponse) error {
	body, err := querier.FetchRPCWithRotation(ctx, client, rpcURLs, fmt.Sprintf("/block?height=%d", height), cfg.GetQueryTimeout())
	if err != nil {
		return err
	}

	var blockRes GetBlockResponse
	if err := json.Unmarshal(body, &blockRes); err != nil {
		return err
	}

	getBlockRes <- blockRes
	return nil
}

func fetchBlockResults(ctx context.Context, client *fiber.Client, height int64, cfg *config.Config, rpcURLs []string, getBlockResultsRes chan<- GetBlockResultsResponse) error {
	body, err := querier.FetchRPCWithRotation(ctx, client, rpcURLs, fmt.Sprintf("/block_results?height=%d", height), cfg.GetQueryTimeout())
	if err != nil {
		return err
	}

	var blockResultsRes GetBlockResultsResponse
	if err := cbjson.Unmarshal(body, &blockResultsRes); err != nil {
		return err
	}

	getBlockResultsRes <- blockResultsRes
	return nil
}

func parseScrapedBlock(block GetBlockResponse, blockResults GetBlockResultsResponse, height int64) (scrapedBlock types.ScrapedBlock, err error) {
	timestamp, err := time.Parse(layout, block.Result.Block.Header.Time)
	if err != nil {
		return scrapedBlock, err
	}

	proposer, err := sdk.ValAddressFromHex(block.Result.Block.Header.ProposerAddress)
	if err != nil {
		return scrapedBlock, err
	}

	var preEvents []abci.Event
	var beginEvents []abci.Event
	var endEvents []abci.Event
	for _, event := range blockResults.Result.FinalizeBlockEvents {
		if len(event.Attributes) == 0 {
			continue
		}

		lastAttr := event.Attributes[len(event.Attributes)-1]
		if lastAttr.Key != "mode" {
			preEvents = append(preEvents, event) // in case of chain upgrade
			continue
		}

		switch lastAttr.Value {
		case "BeginBlock":
			beginEvents = append(beginEvents, event)
		case "EndBlock":
			endEvents = append(endEvents, event)
		}
	}

	return types.ScrapedBlock{
		ChainId:    block.Result.Block.Header.ChainId,
		Height:     height,
		Timestamp:  timestamp,
		Hash:       block.Result.BlockId.Hash,
		Proposer:   proposer.String(),
		Txs:        block.Result.Block.Data.Txs,
		TxResults:  blockResults.Result.TxsResults,
		PreBlock:   preEvents,
		BeginBlock: beginEvents,
		EndBlock:   endEvents,
	}, nil
}
