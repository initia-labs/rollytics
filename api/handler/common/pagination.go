package common

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Pagination struct {
	Limit  int
	Offset int
	Order  string
}

type PaginationResponse struct {
	NextKey *string `json:"next_key" extensions:"x-order:0"`
	Total   string  `json:"total" extensions:"x-order:1"`
}

func ParsePagination(c *fiber.Ctx) (*Pagination, error) {
	limit := c.QueryInt("pagination.limit", 100)
	if limit < 1 || limit > 100 {
		return nil, errors.New("pagination.limit must be between 1 and 100")
	}

	key := c.Query("pagination.key")
	offset := c.QueryInt("pagination.offset", 0)
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
	order := "DESC"
	if !reverse {
		order = "ASC"
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
		nextKey := base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(p.Offset + p.Limit)))
		res.NextKey = &nextKey
	}
	res.Total = fmt.Sprintf("%d", total)
	return
}
