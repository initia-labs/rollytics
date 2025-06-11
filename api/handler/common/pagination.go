package common

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type PaginationParams struct {
	Key        string `query:"pagination.key" extensions:"x-order:0"`
	Offset     uint64 `query:"pagination.offset" extensions:"x-order:1"`
	Limit      uint64 `query:"pagination.limit" extensions:"x-order:2"`
	CountTotal bool   `query:"pagination.count_total" extensions:"x-order:3"`
	Reverse    bool   `query:"pagination.reverse" extensions:"x-order:4"`
}

func ExtractPaginationParams(c *fiber.Ctx) (*PaginationParams, error) {
	params := &PaginationParams{
		Key:        c.Query("pagination.key"),
		Offset:     uint64(c.QueryInt("pagination.offset", 0)),
		Limit:      uint64(c.QueryInt("pagination.limit", 100)),
		CountTotal: c.QueryBool("pagination.count_total", true),
		Reverse:    c.QueryBool("pagination.reverse", true),
	}

	if params.Limit == 0 {
		params.Limit = 100
	}

	return params, nil
}

type PageResponse struct {
	NextKey string `json:"next_key,omitempty" extensions:"x-order:0"`
	Total   int64  `json:"total,omitempty" extensions:"x-order:1"`
}

func (params *PaginationParams) ApplyPagination(query *gorm.DB, keys ...string) (*gorm.DB, error) {
	var err error
	for _, key := range keys {
		if params.Reverse {
			query = query.Order(key + " DESC")
		} else {
			query = query.Order(key + " ASC")
		}
	}
	if len(params.Key) > 0 {
		query, err = params.setPageKey(query, keys)
		if err != nil {
			return nil, fmt.Errorf("failed to set page key: %w", err)
		}
	} else if params.Offset > 0 {
		query = query.Offset(int(params.Offset))
	}

	if params.Limit == 0 {
		params.Limit = 100
	}
	query = query.Limit(int(params.Limit))

	return query, nil
}

func (params *PaginationParams) setPageKey(query *gorm.DB, keys []string) (*gorm.DB, error) {
	var op string
	if params.Reverse {
		op = " < "
	} else {
		op = " > "
	}

	decodedKey, err := base64.StdEncoding.DecodeString(params.Key)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(string(decodedKey), "|")

	if len(parts) != len(keys) {
		return nil, fmt.Errorf("invalid key format: expected %d parts, got %d", len(keys), len(parts))
	}

	if len(parts) == 1 {
		whereClause := fmt.Sprintf("%s %s ?", keys[0], op)
		query = query.Where(whereClause, parts[0])
	} else if len(parts) == 2 {
		whereClause := fmt.Sprintf("(%s %s ?) OR (%s = ? AND %s %s ?)",
			keys[0], op, keys[0], keys[1], op)
		query = query.Where(whereClause, parts[0], parts[0], parts[1])
	} else {
		return nil, fmt.Errorf("unreachable code: too many parts in key")
	}

	return query, nil
}

func GetNextKey(values ...any) []byte {
	if len(values) == 0 {
		return nil
	}

	var parts []string
	for _, v := range values {
		parts = append(parts, fmt.Sprintf("%v", v))
	}

	nextKey := strings.Join(parts, "|")
	return []byte(nextKey)
}

func (params *PaginationParams) GetPageResponse(len int, totalQuery *gorm.DB, nextKey any) (*PageResponse, error) {
	resp := PageResponse{}

	if params.CountTotal {
		if err := totalQuery.Count(&resp.Total).Error; err != nil {
			return nil, fiber.NewError(fiber.StatusInternalServerError, "failed to count total items: "+err.Error())
		}
	}

	if len == int(params.Limit) {
		var keyStr string
		if nextKey != nil {
			keyStr = fmt.Sprintf("%v", nextKey)
		}

		if keyStr != "" {
			resp.NextKey = base64.StdEncoding.EncodeToString([]byte(keyStr))
		}
	}

	return &resp, nil
}
