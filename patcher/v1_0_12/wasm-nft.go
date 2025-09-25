package v1_0_12

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"regexp"

	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	wasm_nft "github.com/initia-labs/rollytics/indexer/collector/wasm-nft"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

var (
	parseErrRegex = regexp.MustCompile(`Error parsing into type [^:]+::msg::QueryMsg: unknown variant`)
)

func PatchWasmNFT(tx *gorm.DB, cfg *config.Config, logger *slog.Logger) error {
	patcher := &WasmNFTPatcher{
		db:         tx,
		cfg:        cfg,
		logger:     logger,
		colInfoMap: make(map[string]CollectionInfo),
		blacklist:  make(map[string]bool),
	}
	return patcher.Run()
}

type WasmNFTPatcher struct {
	db         *gorm.DB
	cfg        *config.Config
	logger     *slog.Logger
	blacklist  map[string]bool
	colInfoMap map[string]CollectionInfo
}

func (p *WasmNFTPatcher) Run() error {
	p.logger.Info("[Patch v1.0.12] Starting Wasm NFT data recovery")

	var totalCount int64
	if err := p.db.Model(&types.CollectedTx{}).Count(&totalCount).Error; err != nil {
		return fmt.Errorf("failed to get transaction count: %w", err)
	}
	p.logger.Info("[Patch v1.0.12] Step 1: Scanning transactions", slog.Int64("total", totalCount))

	p.logger.Info("[Patch v1.0.12] Step 2: Clearing Collection and NFT data only")
	if err := p.clearCollection(); err != nil {
		return err
	}
	if err := p.clearNFT(); err != nil {
		return err
	}

	p.logger.Info("[Patch v1.0.12] Step 3: Processing transactions in batches")
	// Process transactions in batches of 10000
	batchSize := 10000
	processedTxCount := 0
	offset := 0

	for offset < int(totalCount) {
		// Get batch of transactions
		var txs []types.CollectedTx
		if err := p.db.Order("sequence ASC").
			Limit(batchSize).
			Offset(offset).
			Find(&txs).Error; err != nil {
			return fmt.Errorf("failed to query transactions at offset %d: %w", offset, err)
		}

		if len(txs) == 0 {
			break
		}

		if len(txs) > 0 {
			if err := p.processTxBatch(txs); err != nil {
				p.logger.Warn("Failed to process batch",
					slog.Int("offset", offset),
					slog.String("error", err.Error()))
			}
			processedTxCount += len(txs)
		}

		offset += batchSize
	}

	// Step 4: Update all collection counts at the end using colInfoMap keys
	p.logger.Info("[Patch v1.0.12] Step 4: Updating collection counts")
	if err := p.updateAllCollectionCounts(); err != nil {
		return err
	}

	var finalNftCount int64
	p.db.Model(&types.CollectedNft{}).Count(&finalNftCount)
	p.logger.Info("[Patch v1.0.12] Wasm NFT data recovery completed",
		slog.Int("processed_txs", processedTxCount),
		slog.Int64("nfts", finalNftCount))
	return nil
}

func (p *WasmNFTPatcher) clearCollection() error {
	if err := p.db.Exec("DELETE FROM nft_collection").Error; err != nil {
		return fmt.Errorf("failed to clear nft_collection table: %w", err)
	}
	return nil
}

func (p *WasmNFTPatcher) clearNFT() error {
	if err := p.db.Exec("DELETE FROM nft").Error; err != nil {
		return fmt.Errorf("failed to clear nft table: %w", err)
	}

	return nil
}

func (p *WasmNFTPatcher) processTxBatch(txs []types.CollectedTx) error {
	// Maps to track NFT state changes
	mintMap := make(map[util.NftKey]MintInfo)
	transferMap := make(map[util.NftKey]TransferInfo)
	burnMap := make(map[util.NftKey]TxInfo)
	collectionMap := make(map[string]CollectionInfo)
	nftTxMap := make(map[string]map[string]map[string]interface{})

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

		// Process each event
		for _, event := range events {
			eventType, _ := event["type"].(string)
			if eventType != "wasm" {
				continue
			}

			attributes, ok := event["attributes"].([]interface{})
			if !ok {
				continue
			}

			// Convert attributes to map
			attrMap := make(map[string]string)
			for _, attr := range attributes {
				attrObj, ok := attr.(map[string]interface{})
				if !ok {
					continue
				}
				key, _ := attrObj["key"].(string)
				value, _ := attrObj["value"].(string)
				attrMap[key] = value
			}

			// Check if this is a collection event (same logic as original)
			collectionAddr, found := attrMap["_contract_address"]
			if p.blacklist[collectionAddr] {
				continue
			}

			if found && wasm_nft.IsValidCollectionEvent(attrMap) {
				if _, found := collectionMap[collectionAddr]; !found {
					collectionMap[collectionAddr] = CollectionInfo{
						TxInfo: txInfo,
					}
				}
			}

			// Process based on action
			txHash := util.BytesToHexWithPrefix(tx.Hash)
			p.processWasmEvent(attrMap, txInfo, txHash, mintMap, transferMap, burnMap, nftTxMap)
		}
	}

	// Query collection data for detected collections
	if err := p.queryCollectionData(collectionMap); err != nil {
		p.logger.Warn("Failed to query some collection data", slog.String("error", err.Error()))
	}

	// Save processed collection data first
	if err := p.handleCollections(collectionMap); err != nil {
		return err
	}

	// Save processed NFT data
	if err := p.handleMintNFTs(mintMap); err != nil {
		return err
	}

	if err := p.handleTransferNft(transferMap); err != nil {
		return err
	}

	if err := p.handleBurnedNFTs(burnMap); err != nil {
		return err
	}

	if err := p.handleNftTransations(nftTxMap); err != nil {
		return err
	}

	return nil
}

func (p *WasmNFTPatcher) processWasmEvent(
	attrMap map[string]string,
	txInfo TxInfo,
	txHash string,
	mintMap map[util.NftKey]MintInfo,
	transferMap map[util.NftKey]TransferInfo,
	burnMap map[util.NftKey]TxInfo,
	nftTxMap map[string]map[string]map[string]interface{},
) {
	collectionAddr, found := attrMap["_contract_address"]
	if !found {
		return
	}

	// Convert to hex address
	collectionAddrBytes, err := util.AccAddressFromString(collectionAddr)
	if err != nil {
		return
	}
	collectionAddr = util.BytesToHexWithPrefix(collectionAddrBytes)

	action, found := attrMap["action"]
	if !found {
		return
	}

	switch action {
	case "mint":
		tokenId, found := attrMap["token_id"]
		if !found {
			return
		}
		owner, found := attrMap["owner"]
		if !found {
			return
		}

		nftKey := util.NftKey{
			CollectionAddr: collectionAddr,
			TokenId:        tokenId,
		}

		mintMap[nftKey] = MintInfo{
			Owner:  owner,
			Uri:    attrMap["token_uri"],
			TxInfo: txInfo,
		}
		delete(burnMap, nftKey)

		// Track NFT transaction
		if _, ok := nftTxMap[txHash]; !ok {
			nftTxMap[txHash] = make(map[string]map[string]interface{})
		}
		if _, ok := nftTxMap[txHash][collectionAddr]; !ok {
			nftTxMap[txHash][collectionAddr] = make(map[string]interface{})
		}
		nftTxMap[txHash][collectionAddr][tokenId] = nil

	case "transfer_nft", "send_nft":
		tokenId, found := attrMap["token_id"]
		if !found {
			return
		}
		recipient, found := attrMap["recipient"]
		if !found {
			return
		}

		nftKey := util.NftKey{
			CollectionAddr: collectionAddr,
			TokenId:        tokenId,
		}

		transferMap[nftKey] = TransferInfo{
			Owner:  recipient,
			TxInfo: txInfo,
		}

		// Track NFT transaction
		if _, ok := nftTxMap[txHash]; !ok {
			nftTxMap[txHash] = make(map[string]map[string]interface{})
		}
		if _, ok := nftTxMap[txHash][collectionAddr]; !ok {
			nftTxMap[txHash][collectionAddr] = make(map[string]interface{})
		}
		nftTxMap[txHash][collectionAddr][tokenId] = nil

	case "burn":
		tokenId, found := attrMap["token_id"]
		if !found {
			return
		}

		nftKey := util.NftKey{
			CollectionAddr: collectionAddr,
			TokenId:        tokenId,
		}

		burnMap[nftKey] = txInfo
		delete(mintMap, nftKey)
		delete(transferMap, nftKey)

		// Track NFT transaction
		if _, ok := nftTxMap[txHash]; !ok {
			nftTxMap[txHash] = make(map[string]map[string]interface{})
		}
		if _, ok := nftTxMap[txHash][collectionAddr]; !ok {
			nftTxMap[txHash][collectionAddr] = make(map[string]interface{})
		}
		nftTxMap[txHash][collectionAddr][tokenId] = nil
	}
}

type MintInfo struct {
	Owner  string
	Uri    string
	TxInfo TxInfo
}

func (p *WasmNFTPatcher) handleMintNFTs(mintMap map[util.NftKey]MintInfo) error {
	if len(mintMap) == 0 {
		return nil
	}

	var mintedNfts []types.CollectedNft
	var allAddresses []string

	// Collect all owner addresses
	for _, mintInfo := range mintMap {
		allAddresses = append(allAddresses, mintInfo.Owner)
	}

	// Get account IDs
	accountIdMap, err := util.GetOrCreateAccountIds(p.db, allAddresses, true)
	if err != nil {
		return err
	}

	// Create NFT records
	for nftKey, mintInfo := range mintMap {
		collectionAddrBytes, err := util.AccAddressFromString(nftKey.CollectionAddr)
		if err != nil {
			continue
		}

		ownerId := accountIdMap[mintInfo.Owner]

		mintedNfts = append(mintedNfts, types.CollectedNft{
			CollectionAddr: collectionAddrBytes,
			TokenId:        nftKey.TokenId,
			Height:         mintInfo.TxInfo.Height,
			Timestamp:      mintInfo.TxInfo.Timestamp,
			OwnerId:        ownerId,
			Uri:            mintInfo.Uri,
		})
	}

	if len(mintedNfts) > 0 {
		batchSize := p.cfg.GetDBBatchSize()
		return p.db.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedNfts, batchSize).Error
	}
	return nil
}

func (p *WasmNFTPatcher) handleTransferNft(transferMap map[util.NftKey]TransferInfo) error {
	if len(transferMap) == 0 {
		return nil
	}

	var allAddresses []string
	for _, transfer := range transferMap {
		allAddresses = append(allAddresses, transfer.Owner)
	}

	accountIdMap, err := util.GetOrCreateAccountIds(p.db, allAddresses, true)
	if err != nil {
		return err
	}

	for nftKey, transfer := range transferMap {
		collectionAddrBytes, err := util.AccAddressFromString(nftKey.CollectionAddr)
		if err != nil {
			continue
		}

		ownerId := accountIdMap[transfer.Owner]

		if err := p.db.Model(&types.CollectedNft{}).
			Where("collection_addr = ? AND token_id = ?", collectionAddrBytes, nftKey.TokenId).
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

func (p *WasmNFTPatcher) handleBurnedNFTs(burnMap map[util.NftKey]TxInfo) error {
	if len(burnMap) == 0 {
		return nil
	}

	for nftKey := range burnMap {
		collectionAddrBytes, err := util.AccAddressFromString(nftKey.CollectionAddr)
		if err != nil {
			continue
		}

		if err := p.db.Where("collection_addr = ? AND token_id = ?",
			collectionAddrBytes, nftKey.TokenId).
			Delete(&types.CollectedNft{}).Error; err != nil {
			return err
		}
	}

	return nil
}

func (p *WasmNFTPatcher) handleNftTransations(nftTxMap map[string]map[string]map[string]interface{}) error {
	if len(nftTxMap) == 0 {
		return nil
	}

	for txHash, collectionMap := range nftTxMap {
		if txHash == "" {
			continue
		}

		var keys []util.NftKey
		for collectionAddr, nftMap := range collectionMap {
			for tokenId := range nftMap {
				key := util.NftKey{CollectionAddr: collectionAddr, TokenId: tokenId}
				keys = append(keys, key)
			}
		}

		nftIdMap, err := util.GetOrCreateNftIds(p.db, keys, true)
		if err != nil {
			return err
		}

		var nftIds []int64
		for _, key := range keys {
			if id, ok := nftIdMap[key]; ok {
				nftIds = append(nftIds, id)
			}
		}

		txHashBytes, err := util.HexToBytes(txHash)
		if err != nil {
			log.Printf("[Patch v1.0.12] Failed to decode tx hash: %s, error: %v", txHash, err)
			continue
		}

		if err := p.db.Model(&types.CollectedTx{}).
			Where("hash = ?", txHashBytes).
			Update("nft_ids", pq.Array(nftIds)).Error; err != nil {
			return err
		}
	}

	return nil
}

// Add CollectionInfo type
type CollectionInfo struct {
	Name    string
	Creator string
	TxInfo  TxInfo
}

// Add handleCollections method
func (p *WasmNFTPatcher) handleCollections(collectionMap map[string]CollectionInfo) error {
	if len(collectionMap) == 0 {
		return nil
	}

	var allAddresses []string
	for _, colInfo := range collectionMap {
		allAddresses = append(allAddresses, colInfo.Creator)
	}

	// Get account IDs
	accountIdMap, err := util.GetOrCreateAccountIds(p.db, allAddresses, true)
	if err != nil {
		return err
	}

	var collections []types.CollectedNftCollection
	for collectionAddr, colInfo := range collectionMap {
		collectionAddrBytes, err := util.AccAddressFromString(collectionAddr)
		if err != nil {
			continue
		}

		creatorId := accountIdMap[colInfo.Creator]

		collections = append(collections, types.CollectedNftCollection{
			Addr:      collectionAddrBytes,
			Height:    colInfo.TxInfo.Height,
			Timestamp: colInfo.TxInfo.Timestamp,
			Name:      colInfo.Name,
			CreatorId: creatorId,
			NftCount:  0, // Will be updated later when NFTs are processed
		})
	}

	if len(collections) > 0 {
		batchSize := p.cfg.GetDBBatchSize()
		return p.db.Clauses(orm.DoNothingWhenConflict).CreateInBatches(collections, batchSize).Error
	}
	return nil
}

// queryCollectionData queries collection information from the blockchain (same as original)
func (p *WasmNFTPatcher) queryCollectionData(collectionMap map[string]CollectionInfo) error {
	if collectionMap == nil {
		return nil
	}

	for collectionAddr, colInfo := range collectionMap {
		if _, ok := p.colInfoMap[collectionAddr]; ok || p.blacklist[collectionAddr] {
			continue
		}
		name, creator, err := wasm_nft.GetCollectionData(collectionAddr, p.cfg, colInfo.TxInfo.Height)
		if err != nil {
			if parseErrRegex.MatchString(err.Error()) {
				p.blacklist[collectionAddr] = true
				delete(collectionMap, collectionAddr)
				continue
			}
			return err
		}

		// Update the collection info with queried data
		colInfo.Name = name
		colInfo.Creator = creator
		collectionMap[collectionAddr] = colInfo
		p.colInfoMap[collectionAddr] = colInfo
	}
	return nil
}

// Update all collection counts at the end of processing using colInfoMap keys
func (p *WasmNFTPatcher) updateAllCollectionCounts() error {
	if len(p.colInfoMap) == 0 {
		return nil
	}

	// Convert collection addresses from colInfoMap to bytes
	var updateAddrs [][]byte
	for collectionAddr := range p.colInfoMap {
		collectionAddrBytes, err := util.AccAddressFromString(collectionAddr)
		if err != nil {
			continue
		}
		updateAddrs = append(updateAddrs, collectionAddrBytes)
	}

	if len(updateAddrs) == 0 {
		return nil
	}

	// Query actual NFT counts from the nft table, grouped by collection_addr
	var nftCounts []indexertypes.NftCount
	if err := p.db.Table("nft").
		Select("collection_addr, COUNT(*) as count").
		Where("collection_addr IN ?", updateAddrs).
		Group("collection_addr").
		Scan(&nftCounts).Error; err != nil {
		return err
	}

	// Update collection counts in bulk
	for _, nftCount := range nftCounts {
		if err := p.db.Model(&types.CollectedNftCollection{}).
			Where("addr = ?", nftCount.CollectionAddr).
			Update("nft_count", nftCount.Count).Error; err != nil {
			return err
		}
	}

	p.logger.Info("[Patch v1.0.12] Updated collection counts",
		slog.Int("collections_updated", len(nftCounts)))

	return nil
}
