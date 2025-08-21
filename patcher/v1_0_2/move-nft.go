package v1_0_2

import (
	"encoding/json"
	"fmt"
	"log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	movenft "github.com/initia-labs/rollytics/indexer/collector/move-nft"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func PatchMoveNFT(tx *gorm.DB, cfg *config.Config) error {
	patcher := &MoveNFTPatcher{
		db:  tx,
		cfg: cfg,
	}
	return patcher.Run()
}

type MoveNFTPatcher struct {
	db  *gorm.DB
	cfg *config.Config
}

func (p *MoveNFTPatcher) Run() error {
	log.Println("[Patch v1.0.2] Starting Move NFT data recovery...")

	// Step 1: Get all existing NFTs
	var existingNFTs []types.CollectedNft
	if err := p.db.Find(&existingNFTs).Error; err != nil {
		return fmt.Errorf("failed to fetch existing NFTs: %w", err)
	}

	if len(existingNFTs) == 0 {
		log.Println("[Patch v1.0.2] No existing NFTs found, skipping patch")
		return nil
	}

	log.Printf("[Patch v1.0.2] Found %d existing NFTs to process", len(existingNFTs))

	// Step 2: Get unique NFT addresses and convert to account IDs
	nftAccountMap := make(map[string]int64) // nft_addr -> account_id
	var nftAddresses []string

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

	// Step 3: Query all transactions related to these NFT account IDs
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

	log.Printf("[Patch v1.0.2] Found %d NFT-related transactions", len(txs))

	// Step 4: Clear existing NFT data to rebuild
	if err := p.clearNFTData(); err != nil {
		return err
	}

	// Step 5: Process transactions in batches
	batchSize := 100
	for i := 0; i < len(txs); i += batchSize {
		end := min(i+batchSize, len(txs))

		if err := p.processTxBatch(txs[i:end]); err != nil {
			log.Printf("[Patch v1.0.2] Warning: failed to process batch %d-%d: %v", i, end, err)
			continue
		}

		log.Printf("[Patch v1.0.2] Processed %d/%d transactions", end, len(txs))
	}

	log.Println("[Patch v1.0.2] Move NFT data recovery completed")
	return nil
}

func (p *MoveNFTPatcher) clearNFTData() error {
	log.Println("[Patch v1.0.2] Clearing NFT data for rebuild...")

	if err := p.db.Exec("DELETE FROM nft").Error; err != nil {
		return fmt.Errorf("failed to clear nft table: %w", err)
	}

	if err := p.db.Exec("DELETE FROM nft_collection").Error; err != nil {
		return fmt.Errorf("failed to clear nft_collection table: %w", err)
	}

	return nil
}

func (p *MoveNFTPatcher) processTxBatch(txs []types.CollectedTx) error {
	// Maps to track NFT state changes across the batch
	collectionEvents := []CollectionEventInfo{}
	mintMap := make(map[string]map[string]TxInfo)  // collection -> nft -> tx info
	transferMap := make(map[string]TransferInfo)   // nft -> transfer info
	mutMap := make(map[string]MutationInfo)        // nft -> mutation info
	burnMap := make(map[string]TxInfo)             // nft -> tx info
	updateCountMap := make(map[string]interface{}) // collection -> nil

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
		var events []interface{}
		if err := json.Unmarshal(txData.Events, &events); err != nil {
			continue
		}

		// Process each event in the transaction
		for _, evt := range events {
			eventMap, ok := evt.(map[string]interface{})
			if !ok {
				continue
			}

			typeTag, _ := eventMap["type_tag"].(string)
			data, _ := eventMap["data"].(map[string]interface{})
			if data == nil {
				continue
			}

			dataBytes, err := json.Marshal(data)
			if err != nil {
				continue
			}

			switch typeTag {
			case "0x1::collection::CreateCollectionEvent":
				var event movenft.CreateCollectionEvent
				if json.Unmarshal(dataBytes, &event) == nil {
					collectionEvents = append(collectionEvents, CollectionEventInfo{
						Event:  event,
						TxInfo: txInfo,
					})
				}

			case "0x1::collection::MintEvent":
				var event movenft.NftMintAndBurnEvent
				if json.Unmarshal(dataBytes, &event) == nil {
					if _, ok := mintMap[event.Collection]; !ok {
						mintMap[event.Collection] = make(map[string]TxInfo)
					}
					mintMap[event.Collection][event.Nft] = txInfo
					delete(burnMap, event.Nft)
					updateCountMap[event.Collection] = nil
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
					updateCountMap[event.Collection] = nil
				}
			}
		}
	}

	// Now process all the collected events
	if err := p.saveCollections(collectionEvents); err != nil {
		return err
	}

	if err := p.saveMintedNFTs(mintMap); err != nil {
		return err
	}

	if err := p.updateTransfers(transferMap); err != nil {
		return err
	}

	if err := p.updateMutations(mutMap); err != nil {
		return err
	}

	if err := p.deleteBurnedNFTs(burnMap); err != nil {
		return err
	}

	if err := p.updateNFTCounts(updateCountMap); err != nil {
		return err
	}

	return nil
}

func (p *MoveNFTPatcher) saveCollections(events []CollectionEventInfo) error {
	if len(events) == 0 {
		return nil
	}

	var collections []types.CollectedNftCollection
	var allAddresses []string

	// Collect all creator addresses
	for _, eventInfo := range events {
		creator, err := util.AccAddressFromString(eventInfo.Event.Creator)
		if err != nil {
			continue
		}
		allAddresses = append(allAddresses, creator.String())
	}

	// Get account IDs
	accountIdMap, err := util.GetOrCreateAccountIds(p.db, allAddresses, true)
	if err != nil {
		return err
	}

	// Create collection records
	for _, eventInfo := range events {
		creator, err := util.AccAddressFromString(eventInfo.Event.Creator)
		if err != nil {
			continue
		}

		creatorId := accountIdMap[creator.String()]
		collectionAddr, err := util.HexToBytes(eventInfo.Event.Collection)
		if err != nil {
			continue
		}

		collections = append(collections, types.CollectedNftCollection{
			Addr:       collectionAddr,
			Name:       eventInfo.Event.Name,
			OriginName: eventInfo.Event.Name,
			CreatorId:  creatorId,
			Height:     eventInfo.TxInfo.Height,
			Timestamp:  eventInfo.TxInfo.Timestamp,
		})
	}

	if len(collections) > 0 {
		return p.db.Clauses(orm.DoNothingWhenConflict).CreateInBatches(collections, 100).Error
	}
	return nil
}

func (p *MoveNFTPatcher) saveMintedNFTs(mintMap map[string]map[string]TxInfo) error {
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

func (p *MoveNFTPatcher) updateTransfers(transferMap map[string]TransferInfo) error {
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

func (p *MoveNFTPatcher) updateMutations(mutMap map[string]MutationInfo) error {
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

func (p *MoveNFTPatcher) deleteBurnedNFTs(burnMap map[string]TxInfo) error {
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

//nolint:dupl
func (p *MoveNFTPatcher) updateNFTCounts(updateCountMap map[string]interface{}) error {
	if len(updateCountMap) == 0 {
		return nil
	}

	var updateAddrs [][]byte
	for collectionAddr := range updateCountMap {
		addrBytes, err := util.HexToBytes(collectionAddr)
		if err != nil {
			continue
		}
		updateAddrs = append(updateAddrs, addrBytes)
	}

	if len(updateAddrs) == 0 {
		return nil
	}

	var nftCounts []indexertypes.NftCount
	if err := p.db.Table("nft").
		Select("collection_addr, COUNT(*) as count").
		Where("collection_addr IN ?", updateAddrs).
		Group("collection_addr").
		Scan(&nftCounts).Error; err != nil {
		return err
	}

	for _, nftCount := range nftCounts {
		if err := p.db.Model(&types.CollectedNftCollection{}).
			Where("addr = ?", nftCount.CollectionAddr).
			Update("nft_count", nftCount.Count).Error; err != nil {
			return err
		}
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
