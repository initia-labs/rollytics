package v1_0_2

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func PatchWasmNFT(tx *gorm.DB, cfg *config.Config) error {
	patcher := &WasmNFTPatcher{
		db:  tx,
		cfg: cfg,
	}
	return patcher.Run()
}

type WasmNFTPatcher struct {
	db  *gorm.DB
	cfg *config.Config
}

func (p *WasmNFTPatcher) Run() error {
	log.Println("[Patch v1.0.2] Starting Wasm NFT data recovery...")

	// Step 1: Get total transaction count
	var totalCount int64
	if err := p.db.Model(&types.CollectedTx{}).Count(&totalCount).Error; err != nil {
		return fmt.Errorf("failed to get transaction count: %w", err)
	}

	log.Printf("[Patch v1.0.2] Total transactions to scan: %d", totalCount)

	// Step 2: Clear existing NFT data to rebuild
	if err := p.clearNFTData(); err != nil {
		return err
	}

	// Step 3: Process transactions in batches of 10000
	batchSize := 10000
	processedTxCount := 0
	offset := 0

	for offset < int(totalCount) {
		// Get batch of transactions
		var txs []types.CollectedTx
		if err := p.db.Order("height ASC, sequence ASC").
			Limit(batchSize).
			Offset(offset).
			Find(&txs).Error; err != nil {
			return fmt.Errorf("failed to query transactions at offset %d: %w", offset, err)
		}

		if len(txs) == 0 {
			break
		}

		// Filter for transactions with wasm events
		var wasmTxs []types.CollectedTx
		for _, tx := range txs {
			if p.lookUpWasmNFTEvents(tx) {
				wasmTxs = append(wasmTxs, tx)
			}
		}

		if len(wasmTxs) > 0 {
			if err := p.processTxBatch(wasmTxs); err != nil {
				log.Printf("[Patch v1.0.2] Warning: failed to process batch at offset %d: %v", offset, err)
				// Continue processing next batch even if this one fails
			}
			processedTxCount += len(wasmTxs)
		}

		offset += batchSize
		log.Printf("[Patch v1.0.2] Processed %d/%d transactions (found %d NFT txs in this batch, total NFT txs: %d)",
			min(offset, int(totalCount)), totalCount, len(wasmTxs), processedTxCount)
	}

	log.Println("[Patch v1.0.2] Wasm NFT data recovery completed")
	return nil
}

func (p *WasmNFTPatcher) lookUpWasmNFTEvents(tx types.CollectedTx) bool {
	// Parse tx data to check for wasm NFT events
	var txData types.Tx
	if err := json.Unmarshal(tx.Data, &txData); err != nil {
		return false
	}

	var events []interface{}
	if err := json.Unmarshal(txData.Events, &events); err != nil {
		return false
	}

	// Check for wasm events with NFT actions
	for _, evt := range events {
		eventMap, ok := evt.(map[string]interface{})
		if !ok {
			continue
		}

		eventType, _ := eventMap["type"].(string)
		if eventType != "wasm" {
			continue
		}

		attributes, ok := eventMap["attributes"].([]interface{})
		if !ok {
			continue
		}

		// Check if this is an NFT-related event
		hasAction := false
		for _, attr := range attributes {
			if a, ok := attr.(map[string]interface{}); ok {
				key, _ := a["key"].(string)
				value, _ := a["value"].(string)
				if key == "action" && (value == "instantiate" || value == "mint" || 
					value == "transfer_nft" || value == "send_nft" || value == "burn") {
					hasAction = true
					break
				}
			}
		}

		if hasAction {
			return true
		}
	}

	return false
}

func (p *WasmNFTPatcher) clearNFTData() error {
	log.Println("[Patch v1.0.2] Clearing NFT data for rebuild...")

	if err := p.db.Exec("DELETE FROM nft").Error; err != nil {
		return fmt.Errorf("failed to clear nft table: %w", err)
	}

	// Also clear collections since they might be corrupted
	if err := p.db.Exec("DELETE FROM nft_collection").Error; err != nil {
		return fmt.Errorf("failed to clear nft_collection table: %w", err)
	}

	return nil
}

func (p *WasmNFTPatcher) processTxBatch(txs []types.CollectedTx) error {
	// Maps to track NFT state changes
	collectionMap := make(map[string]CollectionInfo) // collection addr -> info
	mintMap := make(map[util.NftKey]MintInfo)
	transferMap := make(map[util.NftKey]TransferInfo)
	burnMap := make(map[util.NftKey]TxInfo)
	updateCountMap := make(map[string]interface{})
	nftTxMap := make(map[string]map[string]map[string]interface{}) // txHash -> collectionAddr -> tokenId -> nil

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

		// Process each event
		for _, evt := range events {
			eventMap, ok := evt.(map[string]interface{})
			if !ok {
				continue
			}

			eventType, _ := eventMap["type"].(string)
			if eventType != "wasm" {
				continue
			}

			attributes, ok := eventMap["attributes"].([]interface{})
			if !ok {
				continue
			}

			// Convert attributes to map
			attrMap := make(map[string]string)
			for _, attr := range attributes {
				if a, ok := attr.(map[string]interface{}); ok {
					key, _ := a["key"].(string)
					value, _ := a["value"].(string)
					attrMap[key] = value
				}
			}

			// Process based on action
			txHash := util.BytesToHexWithPrefix(tx.Hash)
			p.processWasmEvent(attrMap, txInfo, txHash, collectionMap, mintMap, transferMap, burnMap, updateCountMap, nftTxMap)
		}
	}

	// Save processed data
	if err := p.saveCollections(collectionMap); err != nil {
		return err
	}
	
	if err := p.saveMintedNFTs(mintMap); err != nil {
		return err
	}

	if err := p.updateTransfers(transferMap); err != nil {
		return err
	}

	if err := p.deleteBurnedNFTs(burnMap); err != nil {
		return err
	}

	if err := p.updateNFTCounts(updateCountMap); err != nil {
		return err
	}

	if err := p.updateNFTTransactions(nftTxMap); err != nil {
		return err
	}

	return nil
}

func (p *WasmNFTPatcher) processWasmEvent(
	attrMap map[string]string,
	txInfo TxInfo,
	txHash string,
	collectionMap map[string]CollectionInfo,
	mintMap map[util.NftKey]MintInfo,
	transferMap map[util.NftKey]TransferInfo,
	burnMap map[util.NftKey]TxInfo,
	updateCountMap map[string]interface{},
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
	case "instantiate":
		// Collection creation event
		name := attrMap["name"]
		if name == "" {
			name = attrMap["collection_name"]
		}
		symbol := attrMap["symbol"]
		if symbol == "" {
			symbol = attrMap["collection_symbol"]
		}
		creator := attrMap["minter"]
		if creator == "" {
			creator = attrMap["admin"]
		}
		if creator == "" {
			creator = attrMap["creator"]
		}
		
		// Store collection info
		if _, exists := collectionMap[collectionAddr]; !exists {
			collectionMap[collectionAddr] = CollectionInfo{
				Name:    name,
				Symbol:  symbol,
				Creator: creator,
				TxInfo:  txInfo,
			}
		}
		
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
		updateCountMap[collectionAddr] = nil
		
		// Track NFT transaction
		if _, ok := nftTxMap[txHash]; !ok {
			nftTxMap[txHash] = make(map[string]map[string]interface{})
		}
		if _, ok := nftTxMap[txHash][collectionAddr]; !ok {
			nftTxMap[txHash][collectionAddr] = make(map[string]interface{})
		}
		nftTxMap[txHash][collectionAddr][tokenId] = nil
		
		// Track collection if not already tracked
		if _, exists := collectionMap[collectionAddr]; !exists {
			collectionMap[collectionAddr] = CollectionInfo{
				Name:    "", // Will be filled from contract info if available
				Symbol:  "",
				Creator: owner, // Use the NFT owner as a fallback creator
				TxInfo:  txInfo,
			}
		}

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
		updateCountMap[collectionAddr] = nil
		
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

type CollectionInfo struct {
	Name    string
	Symbol  string
	Creator string
	TxInfo  TxInfo
}

type MintInfo struct {
	Owner  string
	Uri    string
	TxInfo TxInfo
}

func (p *WasmNFTPatcher) saveCollections(collectionMap map[string]CollectionInfo) error {
	if len(collectionMap) == 0 {
		return nil
	}

	var collections []types.CollectedNftCollection
	var allAddresses []string

	// Collect all creator addresses
	for _, info := range collectionMap {
		if info.Creator != "" {
			allAddresses = append(allAddresses, info.Creator)
		}
	}

	// Get account IDs for creators
	accountIdMap, err := util.GetOrCreateAccountIds(p.db, allAddresses, true)
	if err != nil {
		return err
	}

	for collectionAddr, info := range collectionMap {
		collectionAddrBytes, err := util.AccAddressFromString(collectionAddr)
		if err != nil {
			continue
		}

		creatorId := int64(0)
		if info.Creator != "" {
			creatorId = accountIdMap[info.Creator]
		}

		collections = append(collections, types.CollectedNftCollection{
			Addr:      collectionAddrBytes,
			Name:      info.Name,
			Height:    info.TxInfo.Height,
			Timestamp: info.TxInfo.Timestamp,
			CreatorId: creatorId,
		})
	}

	if len(collections) > 0 {
		batchSize := p.cfg.GetDBBatchSize()
		return p.db.Clauses(orm.DoNothingWhenConflict).CreateInBatches(collections, batchSize).Error
	}
	return nil
}

func (p *WasmNFTPatcher) saveMintedNFTs(mintMap map[util.NftKey]MintInfo) error {
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

func (p *WasmNFTPatcher) updateTransfers(transferMap map[util.NftKey]TransferInfo) error {
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

func (p *WasmNFTPatcher) deleteBurnedNFTs(burnMap map[util.NftKey]TxInfo) error {
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

func (p *WasmNFTPatcher) updateNFTCounts(updateCountMap map[string]interface{}) error {
	if len(updateCountMap) == 0 {
		return nil
	}

	var updateAddrs [][]byte
	for collectionAddr := range updateCountMap {
		addrBytes, err := util.AccAddressFromString(collectionAddr)
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

func (p *WasmNFTPatcher) updateNFTTransactions(nftTxMap map[string]map[string]map[string]interface{}) error {
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
			log.Printf("[Patch v1.0.2] Failed to decode tx hash: %s, error: %v", txHash, err)
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
