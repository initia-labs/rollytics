// api/handler/common/pagination.go
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
	Offset     int    `query:"pagination.offset" extensions:"x-order:1"`
	Limit      int    `query:"pagination.limit" extensions:"x-order:2"`
	CountTotal bool   `query:"pagination.count_total" extensions:"x-order:3"`
	Reverse    bool   `query:"pagination.reverse" extensions:"x-order:4"`
}

type PageResponse struct {
	NextKey *string `json:"next_key" extensions:"x-order:0"`
	Total   int64   `json:"total,omitempty" extensions:"x-order:1"`
}

func ExtractPaginationParams(c *fiber.Ctx) *PaginationParams {
	params := &PaginationParams{
		Key:        c.Query("pagination.key"),
		Offset:     c.QueryInt("pagination.offset", 0),
		Limit:      c.QueryInt("pagination.limit", 100),
		CountTotal: c.QueryBool("pagination.count_total", true),
		Reverse:    c.QueryBool("pagination.reverse", true),
	}

	if params.Limit == 0 {
		params.Limit = 100
	}

	return params
}

// Apply applies order, limit, and pagination (cursor/offset) to the query
func (params *PaginationParams) Apply(query *gorm.DB, keys ...string) (*gorm.DB, error) {
	// order
	query = params.ApplyOrder(query, keys...)
	// pagination
	query, err := params.ApplyPagination(query, keys...)
	if err != nil {
		return nil, fmt.Errorf("invalid pagination params: %w", err)
	}
	// limit
	query = params.ApplyLimit(query)

	return query, nil
}

func (params *PaginationParams) ApplyLimit(query *gorm.DB) *gorm.DB {
	if params.Limit == 0 {
		params.Limit = 100
	}
	query = query.Limit(int(params.Limit))
	return query
}

func (params *PaginationParams) ApplyPagination(query *gorm.DB, keys ...string) (*gorm.DB, error) {
	var err error
	if params.Key != "" {
		query, err = params.applyCursorPagination(query, keys...)
		if err != nil {
			return nil, err
		}
	} else if params.Offset > 0 {
		query = query.Offset(int(params.Offset))
	}
	return query, nil
}

func (params *PaginationParams) ApplyOrder(query *gorm.DB, keys ...string) *gorm.DB {
	for _, key := range keys {
		if params.Reverse {
			query = query.Order(key + " DESC")
		} else {
			query = query.Order(key + " ASC")
		}
	}
	return query
}

func (params *PaginationParams) applyCursorPagination(query *gorm.DB, keys ...string) (*gorm.DB, error) {
	// decode the base64 key
	decodedKey, err := base64.StdEncoding.DecodeString(params.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to decode pagination key: %w", err)
	}

	parts := strings.Split(string(decodedKey), "|")
	if len(parts) != len(keys) {
		return nil, fmt.Errorf("invalid key format: expected %d parts, got %d", len(keys), len(parts))
	}

	var op string
	if params.Reverse {
		op = " < "
	} else {
		op = " > "
	}

	// build the where based on the keys
	switch len(parts) {
	case 1:
		whereClause := fmt.Sprintf("%s %s ?", keys[0], op)
		query = query.Where(whereClause, parts[0])
	case 2:
		whereClause := fmt.Sprintf("(%s %s ?) OR (%s = ? AND %s %s ?)",
			keys[0], op, keys[0], keys[1], op)
		query = query.Where(whereClause, parts[0], parts[0], parts[1])
	default:
		return nil, fmt.Errorf("unsupported key format: maximum 2 parts supported, got %d", len(parts))
	}

	return query, nil
}


// response method to generate a pagination response
func GetPageResponse[T any](
	params *PaginationParams,
	results []T,
	keyExtractor func(T) []any,
	totalQuery func() int64,
) *PageResponse {
	resp := PageResponse{}
	if params.CountTotal && totalQuery != nil {
		resp.Total = totalQuery()
	}

	if len(results) == int(params.Limit) && len(results) > 0 && keyExtractor != nil {
		lastResult := results[len(results)-1]
		values := keyExtractor(lastResult)
		nextKey := getNextKey(values...)

		if nextKey != nil {
			encodedKey := base64.StdEncoding.EncodeToString(nextKey)
			resp.NextKey = &encodedKey
		}
	}

	return &resp
}

// generate a next key based on the values provided
// it will concatenate the values with a pipe (|) and return a base64 encoded string
func getNextKey(values ...any) []byte {
	if len(values) == 0 {
		return nil
	} else if len(values) == 1 {
		return fmt.Appendf(nil, "%v", values[0])
	}

	var parts []string
	for _, v := range values {
		parts = append(parts, fmt.Sprintf("%v", v))
	}

	nextKey := strings.Join(parts, "|")
	return []byte(nextKey)
}
