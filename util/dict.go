package util

import (
	"errors"

	"github.com/initia-labs/rollytics/cache"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

var (
	msgTypeCache = cache.New[string, int64](10000)
	typeTagCache = cache.New[string, int64](10000)
)

func GetOrCreateMsgTypeIds(db *gorm.DB, msgTypes []string, createNew bool) (ids []int64, err error) {
	for _, msgType := range msgTypes {
		if id, ok := msgTypeCache.Get(msgType); ok {
			ids = append(ids, id)
			continue
		}

		var entry types.CollectedMsgType
		err = db.Where("msg_type = ?", msgType).First(&entry).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if !createNew {
				continue
			}

			entry = types.CollectedMsgType{MsgType: msgType}
			if err := db.Create(&entry).Error; err != nil {
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

func GetOrCreateTypeTagIds(db *gorm.DB, typeTags []string, createNew bool) (ids []int64, err error) {
	for _, typeTag := range typeTags {
		if id, ok := typeTagCache.Get(typeTag); ok {
			ids = append(ids, id)
			continue
		}

		var entry types.CollectedTypeTag
		err = db.Where("type_tag = ?", typeTag).First(&entry).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if !createNew {
				continue
			}

			entry = types.CollectedTypeTag{TypeTag: typeTag}
			if err := db.Create(&entry).Error; err != nil {
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
