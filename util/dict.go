package util

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/cache"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

type NftKey struct {
	CollectionAddr string
	TokenId        string
}

var (
	accountCache          *cache.Cache[string, int64]
	nftCache              *cache.Cache[NftKey, int64]
	msgTypeCache          *cache.Cache[string, int64]
	typeTagCache          *cache.Cache[string, int64]
	evmTxHashCache        *cache.Cache[string, int64]
	evmDenomContractCache *cache.Cache[string, string]

	// Singleton initialization
	cacheInitOnce sync.Once
)

// InitializeCaches initializes all dictionary caches with the given configuration
// This function is safe to call multiple times - it will only initialize once
func InitializeCaches(cfg *config.CacheConfig) {
	cacheInitOnce.Do(func() {
		accountCache = cache.New[string, int64](cfg.AccountCacheSize)
		nftCache = cache.New[NftKey, int64](cfg.NftCacheSize)
		msgTypeCache = cache.New[string, int64](cfg.MsgTypeCacheSize)
		typeTagCache = cache.New[string, int64](cfg.TypeTagCacheSize)
		evmTxHashCache = cache.New[string, int64](cfg.EvmTxHashCacheSize)
		evmDenomContractCache = cache.New[string, string](cfg.EvmDenomContractCacheSize)
	})
}

// checkAccountCache checks the cache for accounts and returns cached IDs and uncached accounts
func checkAccountCache(accounts []string) (idMap map[string]int64, uncached []string) {
	idMap = make(map[string]int64, len(accounts))
	for _, account := range accounts {
		key, err := normalizeAccountToBech32(account)
		if err != nil {
			uncached = append(uncached, account)
			continue
		}
		id, ok := accountCache.Get(key)
		if ok {
			idMap[account] = id
		} else {
			uncached = append(uncached, account)
		}
	}
	return idMap, uncached
}

// fetchAccountsFromDB queries the database for uncached accounts and returns a map of account -> id
func fetchAccountsFromDB(db *gorm.DB, uncached []string) (map[string]int64, error) {
	var entries []types.CollectedAccountDict
	// Convert strings to bytes for the query
	var uncachedBytes [][]byte
	for _, account := range uncached {
		accBytes, err := AccAddressFromString(account)
		if err != nil {
			return nil, err
		}
		uncachedBytes = append(uncachedBytes, accBytes)
	}

	if err := db.Where("account IN ?", uncachedBytes).Find(&entries).Error; err != nil {
		return nil, err
	}

	accountIdMap := make(map[string]int64) // account -> id
	for _, entry := range entries {
		// Use the bech32 string representation as the map key
		accAddr := sdk.AccAddress(entry.Account)
		accountIdMap[accAddr.String()] = entry.Id
	}
	return accountIdMap, nil
}

// createNewAccountEntries creates new account entries in the database if they don't exist
func createNewAccountEntries(db *gorm.DB, uncached []string, accountIdMap map[string]int64) error {
	var newEntries []types.CollectedAccountDict
	for _, account := range uncached {
		if _, ok := accountIdMap[account]; !ok {
			accBytes, err := AccAddressFromString(account)
			if err != nil {
				return err
			}
			newEntries = append(newEntries, types.CollectedAccountDict{Account: accBytes})
		}
	}

	if len(newEntries) > 0 {
		if err := db.Clauses(orm.DoNothingWhenConflict).Create(&newEntries).Error; err != nil {
			return err
		}
		for i, entry := range newEntries {
			accAddr := sdk.AccAddress(entry.Account)
			accountIdMap[accAddr.String()] = newEntries[i].Id
		}
	}
	return nil
}

// updateAccountCacheAndResult updates the cache and result map with found account IDs
func updateAccountCacheAndResult(uncached []string, accountIdMap map[string]int64, idMap map[string]int64) {
	for _, account := range uncached {
		key, err := normalizeAccountToBech32(account)
		if err != nil {
			continue
		}

		if id, ok := accountIdMap[key]; ok {
			accountCache.Set(key, id)
			idMap[account] = id
		}
	}
}

func normalizeAccountToBech32(account string) (string, error) {
	accBytes, err := AccAddressFromString(account)
	if err != nil {
		return "", err
	}
	return sdk.AccAddress(accBytes).String(), nil
}

func GetOrCreateAccountIds(db *gorm.DB, accounts []string, createNew bool) (idMap map[string]int64, err error) {
	// Check cache and collect uncached accounts
	idMap, uncached := checkAccountCache(accounts)

	if len(uncached) == 0 {
		return idMap, nil
	}

	// Fetch existing accounts from database
	accountIdMap, err := fetchAccountsFromDB(db, uncached)
	if err != nil {
		return idMap, err
	}

	// Create new entries if requested
	if createNew {
		if err := createNewAccountEntries(db, uncached, accountIdMap); err != nil {
			return idMap, err
		}
	}

	// Update cache and result map
	updateAccountCacheAndResult(uncached, accountIdMap, idMap)

	return idMap, nil
}

// checkNftCache checks the cache for NFT keys and returns cached IDs and uncached keys
func checkNftCache(keys []NftKey) (idMap map[NftKey]int64, uncached []NftKey) {
	idMap = make(map[NftKey]int64, len(keys))
	for _, key := range keys {
		id, ok := nftCache.Get(key)
		if ok {
			idMap[key] = id
		} else {
			uncached = append(uncached, key)
		}
	}
	return idMap, uncached
}

// fetchNftsFromDB queries the database for uncached NFT keys and returns a map of key -> id
func fetchNftsFromDB(db *gorm.DB, uncached []NftKey) (map[NftKey]int64, error) {
	tx := db.Model(&types.CollectedNftDict{})
	for i, key := range uncached {
		// Convert collection address to bytes
		colAddrBytes, err := AccAddressFromString(key.CollectionAddr)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			tx = tx.Where("collection_addr = ? AND token_id = ?", colAddrBytes, key.TokenId)
		} else {
			tx = tx.Or("collection_addr = ? AND token_id = ?", colAddrBytes, key.TokenId)
		}
	}

	var entries []types.CollectedNftDict
	if err := tx.Find(&entries).Error; err != nil {
		return nil, err
	}

	nftIdMap := make(map[NftKey]int64) // nft key -> id
	for _, entry := range entries {
		// Convert bytes back to string for map key
		key := NftKey{
			CollectionAddr: BytesToHexWithPrefix(entry.CollectionAddr),
			TokenId:        entry.TokenId,
		}
		nftIdMap[key] = entry.Id
	}
	return nftIdMap, nil
}

// createNewNftEntries creates new NFT entries in the database if they don't exist
func createNewNftEntries(db *gorm.DB, uncached []NftKey, nftIdMap map[NftKey]int64) error {
	var newEntries []types.CollectedNftDict
	for _, key := range uncached {
		if _, ok := nftIdMap[key]; !ok {
			// Convert collection address to bytes
			colAddrBytes, err := AccAddressFromString(key.CollectionAddr)
			if err != nil {
				return err
			}
			newEntries = append(newEntries, types.CollectedNftDict{
				CollectionAddr: colAddrBytes,
				TokenId:        key.TokenId,
			})
		}
	}

	if len(newEntries) > 0 {
		if err := db.Clauses(orm.DoNothingWhenConflict).Create(&newEntries).Error; err != nil {
			return err
		}
		for _, entry := range newEntries {
			key := NftKey{
				CollectionAddr: BytesToHexWithPrefix(entry.CollectionAddr),
				TokenId:        entry.TokenId,
			}
			nftIdMap[key] = entry.Id
		}
	}
	return nil
}

// updateNftCacheAndResult updates the cache and result map with found NFT IDs
func updateNftCacheAndResult(uncached []NftKey, nftIdMap map[NftKey]int64, idMap map[NftKey]int64) {
	for _, key := range uncached {
		if id, ok := nftIdMap[key]; ok {
			nftCache.Set(key, id)
			idMap[key] = id
		}
	}
}

func GetOrCreateNftIds(db *gorm.DB, keys []NftKey, createNew bool) (idMap map[NftKey]int64, err error) {
	// Check cache and collect uncached NFT keys
	idMap, uncached := checkNftCache(keys)

	if len(uncached) == 0 {
		return idMap, nil
	}

	// Fetch existing NFTs from database
	nftIdMap, err := fetchNftsFromDB(db, uncached)
	if err != nil {
		return idMap, err
	}

	// Create new entries if requested
	if createNew {
		if err := createNewNftEntries(db, uncached, nftIdMap); err != nil {
			return idMap, err
		}
	}

	// Update cache and result map
	updateNftCacheAndResult(uncached, nftIdMap, idMap)

	return idMap, nil
}

//nolint:dupl
func GetOrCreateMsgTypeIds(db *gorm.DB, msgTypes []string, createNew bool) (idMap map[string]int64, err error) {
	idMap = make(map[string]int64, len(msgTypes))

	// check cache and collect uncached
	var uncached []string
	for _, msgType := range msgTypes {
		id, ok := msgTypeCache.Get(msgType)
		if ok {
			idMap[msgType] = id
		} else {
			uncached = append(uncached, msgType)
		}
	}

	if len(uncached) == 0 {
		return idMap, nil
	}

	// fetch from db to create msgTypeIdMap
	var entries []types.CollectedMsgTypeDict
	if err := db.Where("msg_type IN ?", uncached).Find(&entries).Error; err != nil {
		return idMap, err
	}
	msgTypeIdMap := make(map[string]int64) // msg type -> id
	for _, entry := range entries {
		msgTypeIdMap[entry.MsgType] = entry.Id
	}

	if createNew {
		// create new entries if not in DB
		var newEntries []types.CollectedMsgTypeDict
		for _, msgType := range uncached {
			if _, ok := msgTypeIdMap[msgType]; !ok {
				newEntries = append(newEntries, types.CollectedMsgTypeDict{MsgType: msgType})
			}
		}

		if len(newEntries) > 0 {
			if err := db.Clauses(orm.DoNothingWhenConflict).Create(&newEntries).Error; err != nil {
				return idMap, err
			}
			// Add newly created entries to the map
			for i, entry := range newEntries {
				msgTypeIdMap[entry.MsgType] = newEntries[i].Id
			}
		}
	}

	// set cache and add to result map
	for _, msgType := range uncached {
		if id, ok := msgTypeIdMap[msgType]; ok {
			msgTypeCache.Set(msgType, id)
			idMap[msgType] = id
		}
	}

	return idMap, nil
}

//nolint:dupl
func GetOrCreateTypeTagIds(db *gorm.DB, typeTags []string, createNew bool) (idMap map[string]int64, err error) {
	idMap = make(map[string]int64, len(typeTags))

	// check cache and collect uncached
	var uncached []string
	for _, typeTag := range typeTags {
		id, ok := typeTagCache.Get(typeTag)
		if ok {
			idMap[typeTag] = id
		} else {
			uncached = append(uncached, typeTag)
		}
	}

	if len(uncached) == 0 {
		return idMap, nil
	}

	// fetch from db to create typeTagIdMap
	var entries []types.CollectedTypeTagDict
	if err := db.Where("type_tag IN ?", uncached).Find(&entries).Error; err != nil {
		return idMap, err
	}
	typeTagIdMap := make(map[string]int64) // type tag -> id
	for _, entry := range entries {
		typeTagIdMap[entry.TypeTag] = entry.Id
	}

	if createNew {
		// create new entries if not in DB
		var newEntries []types.CollectedTypeTagDict
		for _, typeTag := range uncached {
			if _, ok := typeTagIdMap[typeTag]; !ok {
				newEntries = append(newEntries, types.CollectedTypeTagDict{TypeTag: typeTag})
			}
		}

		if len(newEntries) > 0 {
			if err := db.Clauses(orm.DoNothingWhenConflict).Create(&newEntries).Error; err != nil {
				return idMap, err
			}
			// Add newly created entries to the map
			for i, entry := range newEntries {
				typeTagIdMap[entry.TypeTag] = newEntries[i].Id
			}
		}
	}

	// set cache and add to result map
	for _, typeTag := range uncached {
		if id, ok := typeTagIdMap[typeTag]; ok {
			typeTagCache.Set(typeTag, id)
			idMap[typeTag] = id
		}
	}

	return idMap, nil
}

func GetOrCreateEvmTxHashIds(db *gorm.DB, hashes [][]byte, createNew bool) (idMap map[string]int64, err error) {
	idMap = make(map[string]int64, len(hashes))

	// check cache and collect uncached
	var uncached [][]byte
	for _, hash := range hashes {
		hashHex := BytesToHex(hash)
		if id, ok := evmTxHashCache.Get(hashHex); ok {
			idMap[hashHex] = id
		} else {
			uncached = append(uncached, hash)
		}
	}

	if len(uncached) == 0 {
		return idMap, nil
	}

	// fetch from db to create hashIdMap
	var entries []types.CollectedEvmTxHashDict
	if err := db.Where("hash IN ?", uncached).Find(&entries).Error; err != nil {
		return idMap, err
	}
	hashIdMap := make(map[string]int64) // hash hex -> id
	for _, entry := range entries {
		hashHex := BytesToHex(entry.Hash)
		hashIdMap[hashHex] = entry.Id
	}

	if createNew {
		// create new entries if not in DB
		var newEntries []types.CollectedEvmTxHashDict
		for _, hash := range uncached {
			hashHex := BytesToHex(hash)
			if _, ok := hashIdMap[hashHex]; !ok {
				newEntries = append(newEntries, types.CollectedEvmTxHashDict{Hash: hash})
			}
		}

		if len(newEntries) > 0 {
			if err := db.Clauses(orm.DoNothingWhenConflict).Create(&newEntries).Error; err != nil {
				return idMap, err
			}
			// Add newly created entries to the map
			for i, entry := range newEntries {
				hashHex := BytesToHex(entry.Hash)
				hashIdMap[hashHex] = newEntries[i].Id
			}
		}
	}

	// set cache and add to result map
	for _, hash := range uncached {
		hashHex := BytesToHex(hash)
		if id, ok := hashIdMap[hashHex]; ok {
			evmTxHashCache.Set(hashHex, id)
			idMap[hashHex] = id
		}
	}

	return idMap, nil
}

// EvmContractByDenomResponse represents the response from /minievm/evm/v1/contracts/by_denom
type EvmContractByDenomResponse struct {
	Address string `json:"address"`
}

// GetEvmContractByDenom queries the MiniEVM API for a contract address by denom
// and caches the result. It returns the contract address or an error.
func GetEvmContractByDenom(ctx context.Context, denom string) (string, error) {
	// Check cache first
	if address, ok := evmDenomContractCache.Get(denom); ok {
		return address, nil
	}

	// Query the API
	path := "/minievm/evm/v1/contracts/by_denom"
	params := map[string]string{"denom": denom}

	body, err := Get(ctx, cfg.GetChainConfig().RestUrl, path, params, nil)
	if err != nil {
		return "", fmt.Errorf("failed to query contract by denom %s: %w", denom, err)
	}

	// Parse the response
	var response EvmContractByDenomResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse contract by denom response: %w", err)
	}

	// Validate the response
	if response.Address == "" {
		return "", fmt.Errorf("empty contract address returned for denom %s", denom)
	}

	// Cache the result
	evmDenomContractCache.Set(denom, response.Address)

	return strings.ToLower(response.Address), nil
}
