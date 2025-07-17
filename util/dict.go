package util

import (
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
	accountCache = cache.New[string, int64](10000)
	nftCache     = cache.New[NftKey, int64](10000)
	msgTypeCache = cache.New[string, int64](10000)
	typeTagCache = cache.New[string, int64](10000)
)

//nolint:dupl
func GetOrCreateAccountIds(db *gorm.DB, accounts []string, createNew bool) (ids []int64, err error) {
	ids = make([]int64, 0, len(accounts))

	// check cache and collect uncached
	var uncached []string
	for _, account := range accounts {
		id, ok := accountCache.Get(account)
		if ok {
			ids = append(ids, id)
		} else {
			uncached = append(uncached, account)
		}
	}

	if len(uncached) == 0 {
		return ids, nil
	}

	// fetch from db to create accountIdMap
	var entries []types.CollectedAccountDict
	if err := db.Where("account IN ?", uncached).Find(&entries).Error; err != nil {
		return ids, err
	}
	accountIdMap := make(map[string]int64) // account -> id
	for _, entry := range entries {
		accountIdMap[entry.Account] = entry.Id
	}

	if createNew {
		// create new entries if not in DB
		var newEntries []types.CollectedAccountDict
		for _, account := range uncached {
			if _, ok := accountIdMap[account]; !ok {
				newEntries = append(newEntries, types.CollectedAccountDict{Account: account})
			}
		}

		if len(newEntries) > 0 {
			if err := db.Clauses(orm.DoNothingWhenConflict).Create(&newEntries).Error; err != nil {
				return ids, err
			}
			for _, entry := range newEntries {
				accountIdMap[entry.Account] = entry.Id
			}
		}
	}

	// set cache and append to ids
	for _, account := range uncached {
		if id, ok := accountIdMap[account]; ok {
			accountCache.Set(account, id)
			ids = append(ids, id)
		}
	}

	return ids, nil
}

func GetOrCreateNftIds(db *gorm.DB, keys []NftKey, createNew bool) (ids []int64, err error) {
	ids = make([]int64, 0, len(keys))

	// check cache and collect uncached
	var uncached []NftKey
	for _, key := range keys {
		id, ok := nftCache.Get(key)
		if ok {
			ids = append(ids, id)
		} else {
			uncached = append(uncached, key)
		}
	}

	if len(uncached) == 0 {
		return ids, nil
	}

	// fetch from db to create nftIdMap
	tx := db.Model(&types.CollectedNftDict{})
	for i, key := range uncached {
		if i == 0 {
			tx = tx.Where("collection_addr = ? AND token_id = ?", key.CollectionAddr, key.TokenId)
		} else {
			tx = tx.Or("collection_addr = ? AND token_id = ?", key.CollectionAddr, key.TokenId)
		}
	}

	var entries []types.CollectedNftDict
	if err := tx.Find(&entries).Error; err != nil {
		return ids, err
	}
	nftIdMap := make(map[NftKey]int64) // account -> id
	for _, entry := range entries {
		key := NftKey{
			CollectionAddr: entry.CollectionAddr,
			TokenId:        entry.TokenId,
		}
		nftIdMap[key] = entry.Id
	}

	if createNew {
		// create new entries if not in DB
		var newEntries []types.CollectedNftDict
		for _, key := range uncached {
			if _, ok := nftIdMap[key]; !ok {
				newEntries = append(newEntries, types.CollectedNftDict{
					CollectionAddr: key.CollectionAddr,
					TokenId:        key.TokenId,
				})
			}
		}

		if len(newEntries) > 0 {
			if err := db.Clauses(orm.DoNothingWhenConflict).Create(&newEntries).Error; err != nil {
				return ids, err
			}
			for _, entry := range newEntries {
				key := NftKey{
					CollectionAddr: entry.CollectionAddr,
					TokenId:        entry.TokenId,
				}
				nftIdMap[key] = entry.Id
			}
		}
	}

	// set cache and append to ids
	for _, key := range uncached {
		if id, ok := nftIdMap[key]; ok {
			nftCache.Set(key, id)
			ids = append(ids, id)
		}
	}

	return ids, nil
}

//nolint:dupl
func GetOrCreateMsgTypeIds(db *gorm.DB, msgTypes []string, createNew bool) (ids []int64, err error) {
	ids = make([]int64, 0, len(msgTypes))

	// check cache and collect uncached
	var uncached []string
	for _, msgType := range msgTypes {
		id, ok := msgTypeCache.Get(msgType)
		if ok {
			ids = append(ids, id)
		} else {
			uncached = append(uncached, msgType)
		}
	}

	if len(uncached) == 0 {
		return ids, nil
	}

	// fetch from db to create msgTypeIdMap
	var entries []types.CollectedMsgTypeDict
	if err := db.Where("msg_type IN ?", uncached).Find(&entries).Error; err != nil {
		return ids, err
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
				return ids, err
			}
			for _, entry := range newEntries {
				msgTypeIdMap[entry.MsgType] = entry.Id
			}
		}
	}

	// set cache and append to ids
	for _, msgType := range uncached {
		if id, ok := msgTypeIdMap[msgType]; ok {
			msgTypeCache.Set(msgType, id)
			ids = append(ids, id)
		}
	}

	return ids, nil
}

//nolint:dupl
func GetOrCreateTypeTagIds(db *gorm.DB, typeTags []string, createNew bool) (ids []int64, err error) {
	ids = make([]int64, 0, len(typeTags))

	// check cache and collect uncached
	var uncached []string
	for _, typeTag := range typeTags {
		id, ok := typeTagCache.Get(typeTag)
		if ok {
			ids = append(ids, id)
		} else {
			uncached = append(uncached, typeTag)
		}
	}

	if len(uncached) == 0 {
		return ids, nil
	}

	// fetch from db to create typeTagIdMap
	var entries []types.CollectedTypeTagDict
	if err := db.Where("type_tag IN ?", uncached).Find(&entries).Error; err != nil {
		return ids, err
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
				return ids, err
			}
			for _, entry := range newEntries {
				typeTagIdMap[entry.TypeTag] = entry.Id
			}
		}
	}

	// set cache and append to ids
	for _, typeTag := range uncached {
		if id, ok := typeTagIdMap[typeTag]; ok {
			typeTagCache.Set(typeTag, id)
			ids = append(ids, id)
		}
	}

	return ids, nil
}
