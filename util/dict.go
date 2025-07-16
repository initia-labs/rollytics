package util

import (
	"errors"

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
	for _, account := range accounts {
		if id, ok := accountCache.Get(account); ok {
			ids = append(ids, id)
			continue
		}

		var entry types.CollectedAccountDict
		err = db.Where("account = ?", account).First(&entry).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// createNew flag is used to prevent api queries from spamming and creating new entries that are meaningless
			if !createNew {
				continue
			}

			entry = types.CollectedAccountDict{Account: account}
			if err := db.Clauses(orm.DoNothingWhenConflict).Create(&entry).Error; err != nil {
				return ids, err
			}
		} else if err != nil {
			return ids, err
		}

		accountCache.Set(account, entry.Id)
		ids = append(ids, entry.Id)
	}

	return ids, nil
}

func GetOrCreateNftIds(db *gorm.DB, keys []NftKey, createNew bool) (ids []int64, err error) {
	ids = make([]int64, 0, len(keys))
	for _, key := range keys {
		if id, ok := nftCache.Get(key); ok {
			ids = append(ids, id)
			continue
		}

		var entry types.CollectedNftDict
		err = db.Where("collection_addr = ? AND token_id = ?", key.CollectionAddr, key.TokenId).First(&entry).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// createNew flag is used to prevent api queries from spamming and creating new entries that are meaningless
			if !createNew {
				continue
			}

			entry = types.CollectedNftDict{CollectionAddr: key.CollectionAddr, TokenId: key.TokenId}
			if err := db.Clauses(orm.DoNothingWhenConflict).Create(&entry).Error; err != nil {
				return ids, err
			}
		} else if err != nil {
			return ids, err
		}

		nftCache.Set(key, entry.Id)
		ids = append(ids, entry.Id)
	}

	return ids, nil
}

//nolint:dupl
func GetOrCreateMsgTypeIds(db *gorm.DB, msgTypes []string, createNew bool) (ids []int64, err error) {
	ids = make([]int64, 0, len(msgTypes))
	for _, msgType := range msgTypes {
		if id, ok := msgTypeCache.Get(msgType); ok {
			ids = append(ids, id)
			continue
		}

		var entry types.CollectedMsgTypeDict
		err = db.Where("msg_type = ?", msgType).First(&entry).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// createNew flag is used to prevent api queries from spamming and creating new entries that are meaningless
			if !createNew {
				continue
			}

			entry = types.CollectedMsgTypeDict{MsgType: msgType}
			if err := db.Clauses(orm.DoNothingWhenConflict).Create(&entry).Error; err != nil {
				return ids, err
			}
		} else if err != nil {
			return ids, err
		}

		msgTypeCache.Set(msgType, entry.Id)
		ids = append(ids, entry.Id)
	}

	return ids, nil
}

//nolint:dupl
func GetOrCreateTypeTagIds(db *gorm.DB, typeTags []string, createNew bool) (ids []int64, err error) {
	ids = make([]int64, 0, len(typeTags))
	for _, typeTag := range typeTags {
		if id, ok := typeTagCache.Get(typeTag); ok {
			ids = append(ids, id)
			continue
		}

		var entry types.CollectedTypeTagDict
		err = db.Where("type_tag = ?", typeTag).First(&entry).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// createNew flag is used to prevent api queries from spamming and creating new entries that are meaningless
			if !createNew {
				continue
			}

			entry = types.CollectedTypeTagDict{TypeTag: typeTag}
			if err := db.Clauses(orm.DoNothingWhenConflict).Create(&entry).Error; err != nil {
				return ids, err
			}
		} else if err != nil {
			return ids, err
		}

		typeTagCache.Set(typeTag, entry.Id)
		ids = append(ids, entry.Id)
	}

	return ids, nil
}
