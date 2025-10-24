package evm_nft

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	nft_pair "github.com/initia-labs/rollytics/indexer/collector/nft-pair"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func (sub *EvmNftSubmodule) collect(block indexertypes.ScrapedBlock, tx *gorm.DB) error {
	sub.mtx.Lock()
	cacheData, ok := sub.cache[block.Height]
	delete(sub.cache, block.Height)
	sub.mtx.Unlock()

	if !ok {
		return errors.New("data is not prepared")
	}

	batchSize := sub.cfg.GetDBBatchSize()
	mintMap := make(map[util.NftKey]string)     // NftKey -> owner
	transferMap := make(map[util.NftKey]string) // NftKey -> new owner
	burnMap := make(map[util.NftKey]interface{})
	updateCountMap := make(map[string]interface{})
	nftTxMap := make(map[string]map[string]map[string]interface{})
	events, err := indexerutil.ExtractEvents(block, "evm")
	if err != nil {
		return err
	}

	for _, event := range events {
		for _, attr := range event.Attributes {
			if attr.Key != "log" {
				continue
			}

			var log evmtypes.Log
			if err := json.Unmarshal([]byte(attr.Value), &log); err != nil {
				return err
			}

			if !isEvmNftLog(log) {
				continue
			}

			collectionAddr := strings.ToLower(log.Address)
			from := log.Topics[1]
			to := log.Topics[2]
			toAddr, err := util.AccAddressFromString(to)
			if err != nil {
				return err
			}
			tokenId, err := convertHexStringToDecString(log.Topics[3])
			if err != nil {
				return err
			}

			nftKey := util.NftKey{
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
			}

			switch {
			case from == emptyAddr && to != emptyAddr:
				// handle mint
				mintMap[nftKey] = toAddr.String()
				delete(burnMap, nftKey)
				updateCountMap[collectionAddr] = nil
			case from != emptyAddr && to != emptyAddr:
				// handle transfer
				transferMap[nftKey] = toAddr.String()
			case from != emptyAddr && to == emptyAddr:
				// handle burn
				burnMap[nftKey] = nil
				delete(mintMap, nftKey)
				delete(transferMap, nftKey)
				updateCountMap[collectionAddr] = nil
			default:
				continue
			}

			if _, ok := nftTxMap[event.TxHash]; !ok {
				nftTxMap[event.TxHash] = make(map[string]map[string]interface{})
			}
			if _, ok := nftTxMap[event.TxHash][collectionAddr]; !ok {
				nftTxMap[event.TxHash][collectionAddr] = make(map[string]interface{})
			}
			nftTxMap[event.TxHash][collectionAddr][tokenId] = nil
		}
	}

	var mintedCols []types.CollectedNftCollection
	var mintedNfts []types.CollectedNft

	mintedCollections := make(map[string][]util.NftKey) // collectionAddr -> []NftKey
	var allAddresses []string

	for nftKey, owner := range mintMap {
		mintedCollections[nftKey.CollectionAddr] = append(mintedCollections[nftKey.CollectionAddr], nftKey)
		ownerAccAddr, err := util.AccAddressFromString(owner)
		if err != nil {
			return err
		}
		allAddresses = append(allAddresses, ownerAccAddr.String())
	}

	for _, owner := range transferMap {
		ownerAccAddr, err := util.AccAddressFromString(owner)
		if err != nil {
			return err
		}
		allAddresses = append(allAddresses, ownerAccAddr.String())
	}

	collectionCreationInfos := make(map[string]CollectionCreationInfo)
	for collectionAddr := range mintedCollections {
		_, ok := cacheData.ColNames[collectionAddr]
		if !ok {
			if sub.IsBlacklisted(collectionAddr) {
				continue
			}
			return types.NewNotFoundError(fmt.Sprintf("collection name info for collection address %s", collectionAddr))
		}

		creationInfo, err := getCollectionCreationInfo(block.ChainId, collectionAddr, tx)
		if err != nil {
			return err
		}

		collectionCreationInfos[collectionAddr] = *creationInfo
		allAddresses = append(allAddresses, creationInfo.Creator)
	}

	accountIdMap, err := util.GetOrCreateAccountIds(tx, allAddresses, true)
	if err != nil {
		return err
	}

	for collectionAddr, nftKeys := range mintedCollections {
		name, ok := cacheData.ColNames[collectionAddr]
		if !ok {
			if sub.IsBlacklisted(collectionAddr) {
				continue
			}
			continue
		}

		addrBytes, err := util.HexToBytes(collectionAddr)
		if err != nil {
			return err
		}

		info := collectionCreationInfos[collectionAddr]
		creatorId := accountIdMap[info.Creator]

		mintedCols = append(mintedCols, types.CollectedNftCollection{
			Addr:      addrBytes,
			Height:    info.Height,
			Timestamp: info.Timestamp,
			Name:      name,
			CreatorId: creatorId,
		})

		for _, nftKey := range nftKeys {
			owner := mintMap[nftKey]
			ownerAccAddr, err := util.AccAddressFromString(owner)
			if err != nil {
				return err
			}

			ownerId := accountIdMap[ownerAccAddr.String()]

			mintedNfts = append(mintedNfts, types.CollectedNft{
				CollectionAddr: addrBytes,
				TokenId:        nftKey.TokenId,
				Height:         block.Height,
				Timestamp:      block.Timestamp,
				OwnerId:        ownerId,
				Uri:            cacheData.TokenUris[collectionAddr][nftKey.TokenId],
			})
		}
	}
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedCols, batchSize).Error; err != nil {
		return err
	}
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedNfts, batchSize).Error; err != nil {
		return err
	}

	var transferredNfts []types.CollectedNft

	for nftKey, owner := range transferMap {
		addrBytes, err := util.HexToBytes(nftKey.CollectionAddr)
		if err != nil {
			return err
		}
		ownerBytes, err := util.AccAddressFromString(owner)
		if err != nil {
			return err
		}

		ownerAddr := sdk.AccAddress(ownerBytes).String()
		ownerId := accountIdMap[ownerAddr]

		transferredNfts = append(transferredNfts, types.CollectedNft{
			CollectionAddr: addrBytes,
			TokenId:        nftKey.TokenId,
			OwnerId:        ownerId,
			Height:         block.Height,
			Timestamp:      block.Timestamp,
		})
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "collection_addr"}, {Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"height", "timestamp", "owner_id"}),
	}).CreateInBatches(transferredNfts, batchSize).Error; err != nil {
		return err
	}

	burnedCollections := make(map[string][]string) // collectionAddr -> tokenIds
	for nftKey := range burnMap {
		burnedCollections[nftKey.CollectionAddr] = append(burnedCollections[nftKey.CollectionAddr], nftKey.TokenId)
	}

	for collectionAddr, tokenIds := range burnedCollections {
		addrBytes, err := util.HexToBytes(collectionAddr)
		if err != nil {
			return err
		}
		if err := tx.
			Where("collection_addr = ? AND token_id IN ?", addrBytes, tokenIds).
			Delete(&types.CollectedNft{}).Error; err != nil {
			return err
		}
	}

	var updateAddrs [][]byte
	for collectionAddr := range updateCountMap {
		addrBytes, err := util.HexToBytes(collectionAddr)
		if err != nil {
			return err
		}
		updateAddrs = append(updateAddrs, addrBytes)
	}

	var nftCounts []indexertypes.NftCount
	if err := tx.Table("nft").
		Select("collection_addr, COUNT(*) as count").
		Where("collection_addr IN ?", updateAddrs).
		Group("collection_addr").
		Scan(&nftCounts).Error; err != nil {
		return err
	}

	for _, nftCount := range nftCounts {
		if err := tx.Model(&types.CollectedNftCollection{}).
			Where("addr = ?", nftCount.CollectionAddr).
			Update("nft_count", nftCount.Count).Error; err != nil {
			return err
		}
	}

	var txNftEdges []types.CollectedTxNft

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

		nftIdMap, err := util.GetOrCreateNftIds(tx, keys, true)
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
			sub.logger.Error("Failed to decode tx hash", "txHash", txHash, "error", err)
			continue
		}

		var seqRow struct {
			Sequence int64
		}
		if err := tx.Model(&types.CollectedTx{}).
			Select("sequence").
			Where("hash = ? AND height = ?", txHashBytes, block.Height).
			Take(&seqRow).Error; err != nil {
			return err
		}

		nftSeen := make(map[int64]struct{}, len(nftIds))
		for _, id := range nftIds {
			if _, ok := nftSeen[id]; ok {
				continue
			}
			nftSeen[id] = struct{}{}
			txNftEdges = append(txNftEdges, types.CollectedTxNft{
				NftId:    id,
				Sequence: seqRow.Sequence,
			})
		}
	}

	if len(txNftEdges) > 0 {
		if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(txNftEdges, batchSize).Error; err != nil {
			return err
		}
	}

	return nft_pair.Collect(block, sub.cfg, tx)
}
