package common

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const (
	DefaultLimit  = 100
	MaxLimit      = 1000
	DefaultOffset = 0
	OrderDesc     = "DESC"
	OrderAsc      = "ASC"
)

type Pagination struct {
	Limit  int
	Offset int
	Order  string
}

type PaginationResponse struct {
	PreviousKey *string `json:"previous_key" extensions:"x-order:0"`
	NextKey     *string `json:"next_key" extensions:"x-order:1"`
	Total       string  `json:"total" extensions:"x-order:2"`
}

func ParsePagination(c *fiber.Ctx) (*Pagination, error) {
	limit := c.QueryInt("pagination.limit", DefaultLimit)
	if limit < 1 || limit > MaxLimit {
		return nil, fmt.Errorf("pagination.limit must be between 1 and %d", MaxLimit)
	}

	key := c.Query("pagination.key")
	offset := c.QueryInt("pagination.offset", DefaultOffset)
	if key != "" {
		decoded, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			return nil, errors.New("pagination.key must be a valid base64 encoded string")
		}

		offset, err = strconv.Atoi(string(decoded))
		if err != nil || offset < 0 {
			return nil, errors.New("pagination.key must decode to a nonnegative integer")
		}
	}

	reverse := c.QueryBool("pagination.reverse", true)
	order := OrderDesc
	if !reverse {
		order = OrderAsc
	}

	return &Pagination{
		Limit:  limit,
		Offset: offset,
		Order:  order,
	}, nil
}

func (p *Pagination) OrderBy(keys ...string) string {
	var parts []string
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s %s", key, p.Order))
	}
	return strings.Join(parts, ", ")
}

func (p *Pagination) ToResponse(total int64) (res PaginationResponse) {
	if total > int64(p.Offset+p.Limit) {
		// Add +1 to prevent overlap: ensures next page doesn't include current page's last item
		nextKey := base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(p.Offset + p.Limit + 1)))
		res.NextKey = &nextKey
	}
	// if offset is greater than limit, previousKey can be set
	if p.Offset > 0 && p.Offset > p.Limit {
		previousKey := base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(p.Offset - p.Limit)))
		res.PreviousKey = &previousKey
	}
	res.Total = fmt.Sprintf("%d", total)
	return
}
