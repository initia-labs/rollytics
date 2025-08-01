package util

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/cache"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

type NftKey struct {
	CollectionAddr string
	TokenId        string
}

var (
	accountCache   = cache.New[string, int64](10000)
	nftCache       = cache.New[NftKey, int64](10000)
	msgTypeCache   = cache.New[string, int64](10000)
	typeTagCache   = cache.New[string, int64](10000)
	evmTxHashCache = cache.New[string, int64](10000)
)

func GetOrCreateAccountIds(db *gorm.DB, accounts []string, createNew bool) (idMap map[string]int64, err error) {
	idMap = make(map[string]int64, len(accounts))

	// check cache and collect uncached
	var uncached []string
	for _, account := range accounts {
		id, ok := accountCache.Get(account)
		if ok {
			idMap[account] = id
		} else {
			uncached = append(uncached, account)
		}
	}

	if len(uncached) == 0 {
		return idMap, nil
	}

	// fetch from db to create accountIdMap
	var entries []types.CollectedAccountDict
	// Convert strings to bytes for the query
	var uncachedBytes [][]byte
	for _, account := range uncached {
		accBytes, err := AccAddressFromString(account)
		if err != nil {
			return idMap, err
		}
		uncachedBytes = append(uncachedBytes, accBytes)
	}
	if err := db.Where("account IN ?", uncachedBytes).Find(&entries).Error; err != nil {
		return idMap, err
	}
	accountIdMap := make(map[string]int64) // account -> id
	for _, entry := range entries {
		// Use the bech32 string representation as the map key
		accAddr := sdk.AccAddress(entry.Account)
		accountIdMap[accAddr.String()] = entry.Id
	}

	if createNew { //nolint:nestif
		// create new entries if not in DB
		var newEntries []types.CollectedAccountDict
		for _, account := range uncached {
			if _, ok := accountIdMap[account]; !ok {
				accBytes, err := AccAddressFromString(account)
				if err != nil {
					return idMap, err
				}
				newEntries = append(newEntries, types.CollectedAccountDict{Account: accBytes})
			}
		}

		if len(newEntries) > 0 {
			if err := db.Clauses(orm.DoNothingWhenConflict).Create(&newEntries).Error; err != nil {
				return idMap, err
			}
			for i, entry := range newEntries {
				accAddr := sdk.AccAddress(entry.Account)
				accountIdMap[accAddr.String()] = newEntries[i].Id
			}
		}
	}

	for _, account := range uncached {
		if id, ok := accountIdMap[account]; ok {
			accountCache.Set(account, id)
			idMap[account] = id
		}
	}

	return idMap, nil
}

func GetOrCreateNftIds(db *gorm.DB, keys []NftKey, createNew bool) (idMap map[NftKey]int64, err error) {
	idMap = make(map[NftKey]int64, len(keys))

	// check cache and collect uncached
	var uncached []NftKey
	for _, key := range keys {
		id, ok := nftCache.Get(key)
		if ok {
			idMap[key] = id
		} else {
			uncached = append(uncached, key)
		}
	}

	if len(uncached) == 0 {
		return idMap, nil
	}

	// fetch from db to create nftIdMap
	tx := db.Model(&types.CollectedNftDict{})
	for i, key := range uncached {
		// Convert collection address to bytes
		colAddrBytes, err := AccAddressFromString(key.CollectionAddr)
		if err != nil {
			return idMap, err
		}
		if i == 0 {
			tx = tx.Where("collection_addr = ? AND token_id = ?", colAddrBytes, key.TokenId)
		} else {
			tx = tx.Or("collection_addr = ? AND token_id = ?", colAddrBytes, key.TokenId)
		}
	}

	var entries []types.CollectedNftDict
	if err := tx.Find(&entries).Error; err != nil {
		return idMap, err
	}
	nftIdMap := make(map[NftKey]int64) // nft key -> id
	for _, entry := range entries {
		// Convert bytes back to string for map key
		key := NftKey{
			CollectionAddr: BytesToHex(entry.CollectionAddr),
			TokenId:        entry.TokenId,
		}
		nftIdMap[key] = entry.Id
	}

	if createNew { //nolint:nestif
		// create new entries if not in DB
		var newEntries []types.CollectedNftDict
		for _, key := range uncached {
			if _, ok := nftIdMap[key]; !ok {
				// Convert collection address to bytes
				colAddrBytes, err := AccAddressFromString(key.CollectionAddr)
				if err != nil {
					return idMap, err
				}
				newEntries = append(newEntries, types.CollectedNftDict{
					CollectionAddr: colAddrBytes,
					TokenId:        key.TokenId,
				})
			}
		}

		if len(newEntries) > 0 {
			if err := db.Clauses(orm.DoNothingWhenConflict).Create(&newEntries).Error; err != nil {
				return idMap, err
			}
			// Add newly created entries to the map
			j := 0
			for _, key := range uncached {
				if _, ok := nftIdMap[key]; !ok {
					nftIdMap[key] = newEntries[j].Id
					j++
				}
			}
		}
	}

	// set cache and add to result map
	for _, key := range uncached {
		if id, ok := nftIdMap[key]; ok {
			nftCache.Set(key, id)
			idMap[key] = id
		}
	}

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
