package scrapper

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	cbjson "github.com/cometbft/cometbft/libs/json"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/types"
	"golang.org/x/sync/errgroup"
)

func scrapBlock(client *fiber.Client, height int64, cfg *config.Config) (types.ScrappedBlock, error) {
	var g errgroup.Group
	getBlockRes := make(chan GetBlockResponse, 1)
	getBlockResultsRes := make(chan GetBlockResultsResponse, 1)

	g.Go(func() error {
		defer close(getBlockRes)
		return fetchBlock(client, height, cfg, getBlockRes)
	})

	g.Go(func() error {
		defer close(getBlockResultsRes)
		return fetchBlockResults(client, height, cfg, getBlockResultsRes)
	})

	if err := g.Wait(); err != nil {
		return types.ScrappedBlock{}, err
	}

	block := <-getBlockRes
	blockResults := <-getBlockResultsRes

	return parseScrappedBlock(block, blockResults, height)
}

func fetchBlock(client *fiber.Client, height int64, cfg *config.Config, getBlockRes chan<- GetBlockResponse) error {
	url := fmt.Sprintf("%s/block?height=%d", cfg.GetChainConfig().RpcUrl, height)
	body, err := fetchFromRpc(client, url)
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

func fetchBlockResults(client *fiber.Client, height int64, cfg *config.Config, getBlockResultsRes chan<- GetBlockResultsResponse) error {
	url := fmt.Sprintf("%s/block_results?height=%d", cfg.GetChainConfig().RpcUrl, height)
	body, err := fetchFromRpc(client, url)
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

func fetchFromRpc(client *fiber.Client, url string) (body []byte, err error) {
	code, body, errs := client.Get(url).Timeout(10 * time.Second).Bytes()
	if err := errors.Join(errs...); err != nil {
		return body, err
	}

	if code != fiber.StatusOK {
		if code == fiber.StatusInternalServerError {
			var res RpcErrorResponse
			if err := json.Unmarshal(body, &res); err != nil {
				return body, err
			}

			reHeight := regexp.MustCompile(`current blockchain height (\d+)`)
			heightMatches := reHeight.FindStringSubmatch(res.Error.Data)
			if len(heightMatches) > 1 {
				return body, fmt.Errorf("current height: %s", heightMatches[1])
			}

			reNotFound := regexp.MustCompile(`could not find results for height #(\d+)`)
			notFoundMatches := reNotFound.FindStringSubmatch(res.Error.Data)
			if len(notFoundMatches) > 1 {
				return body, fmt.Errorf("could not find results for height: %s", notFoundMatches[1])
			}
		}

		return body, fmt.Errorf("http response: %d, body: %s", code, string(body))
	}

	return body, nil
}

func parseScrappedBlock(block GetBlockResponse, blockResults GetBlockResultsResponse, height int64) (scrappedBlock types.ScrappedBlock, err error) {
	timestamp, err := time.Parse(layout, block.Result.Block.Header.Time)
	if err != nil {
		return scrappedBlock, err
	}

	proposer, err := sdk.ConsAddressFromHex(block.Result.Block.Header.ProposerAddress)
	if err != nil {
		return scrappedBlock, err
	}

	var beginEvts []abci.Event
	var endEvts []abci.Event
	for _, evt := range blockResults.Result.FinalizeBlockEvents {
		lastAttr := evt.Attributes[len(evt.Attributes)-1]
		if lastAttr.Key != "mode" {
			continue
		}

		if lastAttr.Value == "BeginBlock" {
			beginEvts = append(beginEvts, evt)
		} else if lastAttr.Value == "EndBlock" {
			endEvts = append(endEvts, evt)
		}
	}

	return types.ScrappedBlock{
		ChainId:    block.Result.Block.Header.ChainId,
		Height:     height,
		Timestamp:  timestamp,
		Hash:       block.Result.BlockId.Hash,
		Proposer:   proposer.String(),
		Txs:        block.Result.Block.Data.Txs,
		TxResults:  blockResults.Result.TxsResults,
		BeginBlock: beginEvts,
		EndBlock:   endEvts,
	}, nil
}
