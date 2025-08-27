package v1_0_2

import (
	"encoding/json"
	"fmt"
	"log/slog"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	movenft "github.com/initia-labs/rollytics/indexer/collector/move-nft"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func PatchMoveNFT(tx *gorm.DB, cfg *config.Config, logger *slog.Logger) error {
	patcher := &MoveNFTPatcher{
		db:     tx,
		cfg:    cfg,
		logger: logger,
	}
	return patcher.Run()
}

type MoveNFTPatcher struct {
	db     *gorm.DB
	cfg    *config.Config
	logger *slog.Logger
}

func (p *MoveNFTPatcher) Run() error {
	p.logger.Info("[Patch v1.0.2] Starting Move NFT data recovery")

	p.logger.Info("[Patch v1.0.2] Step 1: Fetching existing NFTs")
	var existingNFTs []types.CollectedNft
	if err := p.db.Find(&existingNFTs).Error; err != nil {
		return fmt.Errorf("failed to fetch existing NFTs: %w", err)
	}

	if len(existingNFTs) == 0 {
		p.logger.Info("[Patch v1.0.2] No existing NFTs found, skipping patch")
		return nil
	}

	p.logger.Info("[Patch v1.0.2] Step 2: Building account ID mappings")
	nftAccountMap := make(map[string]int64) // nft_addr -> account_id
	var nftAddresses []string

	// Collect NFT addresses
	for _, nft := range existingNFTs {
		accAddr := sdk.AccAddress(nft.Addr)
		nftAddresses = append(nftAddresses, accAddr.String())
	}

	// Get or create account IDs for NFT addresses
	accountIdMap, err := util.GetOrCreateAccountIds(p.db, nftAddresses, true)
	if err != nil {
		return fmt.Errorf("failed to get account IDs: %w", err)
	}

	for _, nft := range existingNFTs {
		accAddr := sdk.AccAddress(nft.Addr)
		if accountId, ok := accountIdMap[accAddr.String()]; ok {
			nftAccountMap[util.BytesToHexWithPrefix(nft.Addr)] = accountId
		}
	}

	p.logger.Info("[Patch v1.0.2] Step 3: Querying related transactions")
	var accountIds []int64
	for _, id := range nftAccountMap {
		accountIds = append(accountIds, id)
	}

	var txs []types.CollectedTx
	if err := p.db.Where("account_ids && ?", pq.Array(accountIds)).
		Order("sequence ASC").
		Find(&txs).Error; err != nil {
		return fmt.Errorf("failed to query NFT transactions: %w", err)
	}

	p.logger.Info("[Patch v1.0.2] Step 4: Clearing NFT data only")
	if err := p.clearNFT(); err != nil {
		return err
	}

	p.logger.Info("[Patch v1.0.2] Step 5: Processing transactions in batches",
		slog.Int("total", len(txs)))
	batchSize := 100
	for i := 0; i < len(txs); i += batchSize {
		end := min(i+batchSize, len(txs))
		if err := p.processTxBatch(txs[i:end]); err != nil {
			p.logger.Warn("Failed to process batch",
				slog.Int("start", i),
				slog.Int("end", end),
				slog.String("error", err.Error()))
			continue
		}
	}

	var finalNftCount int64
	p.db.Model(&types.CollectedNft{}).Count(&finalNftCount)
	p.logger.Info("[Patch v1.0.2] Move NFT data recovery completed",
		slog.Int64("nfts", finalNftCount))

	return nil
}

func (p *MoveNFTPatcher) clearNFT() error {
	// Only clear NFT table, don't touch collections at all
	if err := p.db.Exec("DELETE FROM nft").Error; err != nil {
		return fmt.Errorf("failed to clear nft table: %w", err)
	}
	return nil
}

func (p *MoveNFTPatcher) processTxBatch(txs []types.CollectedTx) error {
	// Maps to track NFT state changes across the batch
	mintMap := make(map[string]map[string]TxInfo) // collection -> nft -> tx info
	transferMap := make(map[string]TransferInfo)  // nft -> transfer info
	mutMap := make(map[string]MutationInfo)       // nft -> mutation info
	burnMap := make(map[string]TxInfo)            // nft -> tx info

	// Process each transaction
	for _, tx := range txs {
		// Parse tx data to get timestamp and events
		var txData types.Tx
		if err := json.Unmarshal(tx.Data, &txData); err != nil {
			continue
		}

		txInfo := TxInfo{
			Height:    tx.Height,
			Sequence:  tx.Sequence,
			Timestamp: txData.Timestamp,
		}

		// Parse events
		var events []map[string]interface{}
		if err := json.Unmarshal(txData.Events, &events); err != nil {
			continue
		}

		// Process each event in the transaction
		for _, event := range events {
			eventType, _ := event["type"].(string)

			// We only care about 'move' type events for NFTs
			if eventType != "move" {
				continue
			}

			attributes, ok := event["attributes"].([]interface{})
			if !ok {
				continue
			}

			var typeTag string
			var data map[string]interface{}

			for _, attr := range attributes {
				attrMap, ok := attr.(map[string]interface{})
				if !ok {
					continue
				}

				key, _ := attrMap["key"].(string)
				value, _ := attrMap["value"].(string)

				switch key {
				case "type_tag":
					typeTag = value
				case "data":
					if err := json.Unmarshal([]byte(value), &data); err != nil {
						continue
					}
				}
			}

			if typeTag == "" || data == nil {
				continue
			}

			dataBytes, err := json.Marshal(data)
			if err != nil {
				continue
			}

			switch typeTag {
			case "0x1::collection::MintEvent":
				var event movenft.NftMintAndBurnEvent
				if json.Unmarshal(dataBytes, &event) == nil {
					if _, ok := mintMap[event.Collection]; !ok {
						mintMap[event.Collection] = make(map[string]TxInfo)
					}
					mintMap[event.Collection][event.Nft] = txInfo
					delete(burnMap, event.Nft)
				}

			case "0x1::object::TransferEvent":
				var event movenft.NftTransferEvent
				if json.Unmarshal(dataBytes, &event) == nil {
					toAddr, err := util.AccAddressFromString(event.To)
					if err == nil {
						transferMap[event.Object] = TransferInfo{
							Owner:  toAddr.String(),
							TxInfo: txInfo,
						}
					}
				}

			case "0x1::nft::MutationEvent":
				var event movenft.NftMutationEvent
				if json.Unmarshal(dataBytes, &event) == nil {
					if event.MutatedFieldName == "uri" {
						mutMap[event.Nft] = MutationInfo{
							Uri:    event.NewValue,
							TxInfo: txInfo,
						}
					}
				}

			case "0x1::collection::BurnEvent":
				var event movenft.NftMintAndBurnEvent
				if json.Unmarshal(dataBytes, &event) == nil {
					burnMap[event.Nft] = txInfo
					// Remove from mint map if exists
					if collection, ok := mintMap[event.Collection]; ok {
						delete(collection, event.Nft)
					}
					delete(transferMap, event.Nft)
					delete(mutMap, event.Nft)
				}
			}
		}
	}

	// Process NFT events
	if err := p.handleMintNft(mintMap); err != nil {
		return err
	}

	if err := p.handleTransferNft(transferMap); err != nil {
		return err
	}

	if err := p.handleMutateNft(mutMap); err != nil {
		return err
	}

	if err := p.handleBurnNft(burnMap); err != nil {
		return err
	}

	return nil
}

func (p *MoveNFTPatcher) handleMintNft(mintMap map[string]map[string]TxInfo) error {
	var mintedNfts []types.CollectedNft

	for collectionAddr, nftMap := range mintMap {
		creatorId, err := p.getCollectionCreatorId(collectionAddr)
		if err != nil {
			continue
		}

		collectionAddrBytes, err := util.HexToBytes(collectionAddr)
		if err != nil {
			continue
		}

		for nftAddr, txInfo := range nftMap {
			nftAddrBytes, err := util.HexToBytes(nftAddr)
			if err != nil {
				continue
			}

			mintedNfts = append(mintedNfts, types.CollectedNft{
				CollectionAddr: collectionAddrBytes,
				TokenId:        nftAddr, // For Move, token_id is the nft address
				Addr:           nftAddrBytes,
				Height:         txInfo.Height,
				Timestamp:      txInfo.Timestamp,
				OwnerId:        creatorId,
			})
		}
	}

	if len(mintedNfts) > 0 {
		batchSize := p.cfg.GetDBBatchSize()
		return p.db.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedNfts, batchSize).Error
	}
	return nil
}

func (p *MoveNFTPatcher) handleTransferNft(transferMap map[string]TransferInfo) error {
	if len(transferMap) == 0 {
		return nil
	}

	// Collect all owner addresses
	var allAddresses []string
	for _, transfer := range transferMap {
		allAddresses = append(allAddresses, transfer.Owner)
	}

	// Get account IDs
	accountIdMap, err := util.GetOrCreateAccountIds(p.db, allAddresses, true)
	if err != nil {
		return err
	}

	// Update NFT owners
	for nftAddr, transfer := range transferMap {
		ownerId := accountIdMap[transfer.Owner]
		nftAddrBytes, err := util.HexToBytes(nftAddr)
		if err != nil {
			continue
		}

		if err := p.db.Model(&types.CollectedNft{}).
			Where("addr = ?", nftAddrBytes).
			Updates(map[string]interface{}{
				"height":    transfer.TxInfo.Height,
				"timestamp": transfer.TxInfo.Timestamp,
				"owner_id":  ownerId,
			}).Error; err != nil {
			return err
		}
	}

	return nil
}

func (p *MoveNFTPatcher) handleMutateNft(mutMap map[string]MutationInfo) error {
	for nftAddr, mutation := range mutMap {
		nftAddrBytes, err := util.HexToBytes(nftAddr)
		if err != nil {
			continue
		}

		if err := p.db.Model(&types.CollectedNft{}).
			Where("addr = ?", nftAddrBytes).
			Updates(map[string]interface{}{
				"height":    mutation.TxInfo.Height,
				"timestamp": mutation.TxInfo.Timestamp,
				"uri":       mutation.Uri,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (p *MoveNFTPatcher) handleBurnNft(burnMap map[string]TxInfo) error {
	if len(burnMap) == 0 {
		return nil
	}

	var burnedNfts [][]byte
	for nftAddr := range burnMap {
		burnedNft, err := util.HexToBytes(nftAddr)
		if err != nil {
			continue
		}
		burnedNfts = append(burnedNfts, burnedNft)
	}

	if len(burnedNfts) > 0 {
		return p.db.Where("addr IN ?", burnedNfts).Delete(&types.CollectedNft{}).Error
	}
	return nil
}

func (p *MoveNFTPatcher) getCollectionCreatorId(collectionAddr string) (int64, error) {
	collectionAddrBytes, err := util.HexToBytes(collectionAddr)
	if err != nil {
		return 0, err
	}

	var collection types.CollectedNftCollection
	if err := p.db.Where("addr = ?", collectionAddrBytes).First(&collection).Error; err != nil {
		return 0, err
	}

	return collection.CreatorId, nil
}
