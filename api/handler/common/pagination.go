package common

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

const (
	DefaultLimit  = 100
	MaxLimit      = 1000
	DefaultOffset = 0
	OrderDesc     = "DESC"
	OrderAsc      = "ASC"
)

type CursorType int

const (
	CursorTypeOffset    CursorType = iota + 1 // 1: traditional offset-based pagination
	CursorTypeSequence                        // 2: sequence-based cursor pagination
	CursorTypeHeight                          // 3: height-based cursor pagination
	CursorTypeComposite                       // 4: composite cursor (height + token_id, etc.)
)

// String method for debugging and logging support
func (ct CursorType) String() string {
	switch ct {
	case CursorTypeOffset:
		return "offset"
	case CursorTypeSequence:
		return "sequence"
	case CursorTypeHeight:
		return "height"
	case CursorTypeComposite:
		return "composite"
	default:
		return "unknown"
	}
}

// Generic cursor interface for all record types
type CursorRecord interface {
	GetCursorFields() []string
	GetCursorValue(field string) any
	// Performance optimization: get all cursor data in one call
	GetCursorData() map[string]any
}

type Pagination struct {
	Limit       int
	Offset      int
	Order       string
	CursorType  CursorType
	CursorValue map[string]any
}

// UseCursor determines whether cursor-based pagination is enabled
func (p *Pagination) UseCursor() bool {
	return p.CursorType != CursorTypeOffset && p.CursorType != 0
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

	pagination := &Pagination{
		Limit:      limit,
		Offset:     offset,
		Order:      getOrder(c),
		CursorType: CursorTypeOffset, // default to offset-based pagination
	}

	if key != "" {
		decoded, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			return nil, errors.New("pagination.key must be a valid base64 encoded string")
		}

		// Check if it's JSON cursor format
		if strings.HasPrefix(string(bytes.TrimSpace(decoded)), "{") {
			var cursorData map[string]any
			if json.Unmarshal(decoded, &cursorData) == nil {
				pagination.CursorValue = cursorData
				pagination.CursorType = detectCursorType(cursorData)
			} else {
				// Fallback to traditional approach if JSON parsing fails
				if parsedOffset, err := strconv.Atoi(string(decoded)); err == nil && parsedOffset >= 0 {
					pagination.Offset = parsedOffset
				} else {
					return nil, errors.New("invalid pagination.key format")
				}
			}
		} else {
			// Traditional integer approach
			if parsedOffset, err := strconv.Atoi(string(decoded)); err == nil && parsedOffset >= 0 {
				pagination.Offset = parsedOffset
			} else {
				return nil, errors.New("pagination.key must decode to a nonnegative integer")
			}
		}
	}

	return pagination, nil
}

func getOrder(c *fiber.Ctx) string {
	reverse := c.QueryBool("pagination.reverse", true)
	if reverse {
		return OrderDesc
	}
	return OrderAsc
}

// detectCursorType automatically detects cursor type from cursor data
func detectCursorType(cursorData map[string]any) CursorType {
	if _, hasSequence := cursorData["sequence"]; hasSequence {
		return CursorTypeSequence
	}

	if _, hasHeight := cursorData["height"]; hasHeight {
		if _, hasTokenId := cursorData["token_id"]; hasTokenId {
			return CursorTypeComposite // height + token_id
		}
		return CursorTypeHeight // height only
	}

	return CursorTypeOffset // default
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
	// if offset is greater than or equal to limit, previousKey can be set
	if p.Offset > 0 && p.Offset >= p.Limit {
		previousKey := base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(p.Offset - p.Limit)))
		res.PreviousKey = &previousKey
	}
	res.Total = fmt.Sprintf("%d", total)
	return
}

func (p *Pagination) ToResponseWithLastRecord(total int64, lastRecord any) PaginationResponse {
	if p.UseCursor() && lastRecord != nil {
		if r, ok := lastRecord.(CursorRecord); ok {
			nextCursor := r.GetCursorData()
			if len(nextCursor) > 0 {
				cursorBytes, _ := json.Marshal(nextCursor)
				nextKey := base64.StdEncoding.EncodeToString(cursorBytes)
				return PaginationResponse{
					NextKey: &nextKey,
					Total:   fmt.Sprintf("%d", total),
				}
			}
		}
	}

	// Fall back to traditional offset-based approach
	return p.ToResponse(total)
}

// Query application methods for each table type
func (p *Pagination) ApplyToEvmInternalTx(query *gorm.DB) *gorm.DB {
	switch p.CursorType {
	case CursorTypeSequence:
		sequence := int64(p.CursorValue["sequence"].(float64))
		if p.Order == OrderDesc {
			query = query.Where("sequence < ?", sequence)
		} else {
			query = query.Where("sequence > ?", sequence)
		}
		return query.Order(p.OrderBy("sequence")).Limit(p.Limit)

	case CursorTypeOffset:
		fallthrough
	default:
		return query.Order(p.OrderBy("sequence")).Offset(p.Offset).Limit(p.Limit)
	}
}

func (p *Pagination) ApplyToNftCollection(query *gorm.DB) *gorm.DB {
	switch p.CursorType {
	case CursorTypeHeight:
		height := int64(p.CursorValue["height"].(float64))
		if p.Order == OrderDesc {
			query = query.Where("height < ?", height)
		} else {
			query = query.Where("height > ?", height)
		}
		return query.Order(p.OrderBy("height")).Limit(p.Limit)

	case CursorTypeOffset:
		fallthrough
	default:
		return query.Order(p.OrderBy("height")).Offset(p.Offset).Limit(p.Limit)
	}
}

func (p *Pagination) ApplyToNft(query *gorm.DB) *gorm.DB {
	switch p.CursorType {
	case CursorTypeComposite:
		height := int64(p.CursorValue["height"].(float64))
		tokenId := p.CursorValue["token_id"].(string)

		if p.Order == OrderDesc {
			query = query.Where("(height, token_id) < (?, ?)", height, tokenId)
		} else {
			query = query.Where("(height, token_id) > (?, ?)", height, tokenId)
		}
		return query.Order(p.OrderBy("height", "token_id")).Limit(p.Limit)

	case CursorTypeHeight:
		height := int64(p.CursorValue["height"].(float64))
		if p.Order == OrderDesc {
			query = query.Where("height < ?", height)
		} else {
			query = query.Where("height > ?", height)
		}
		return query.Order(p.OrderBy("height", "token_id")).Limit(p.Limit)

	case CursorTypeOffset:
		fallthrough
	default:
		return query.Order(p.OrderBy("height", "token_id")).Offset(p.Offset).Limit(p.Limit)
	}
}

// Additional Apply methods for other table types
func (p *Pagination) ApplyToTx(query *gorm.DB) *gorm.DB {
	switch p.CursorType {
	case CursorTypeSequence:
		sequence := int64(p.CursorValue["sequence"].(float64))
		if p.Order == OrderDesc {
			query = query.Where("sequence < ?", sequence)
		} else {
			query = query.Where("sequence > ?", sequence)
		}
		return query.Order(p.OrderBy("sequence")).Limit(p.Limit)

	case CursorTypeOffset:
		fallthrough
	default:
		return query.Order(p.OrderBy("sequence")).Offset(p.Offset).Limit(p.Limit)
	}
}

func (p *Pagination) ApplyToEvmTx(query *gorm.DB) *gorm.DB {
	switch p.CursorType {
	case CursorTypeSequence:
		sequence := int64(p.CursorValue["sequence"].(float64))
		if p.Order == OrderDesc {
			query = query.Where("sequence < ?", sequence)
		} else {
			query = query.Where("sequence > ?", sequence)
		}
		return query.Order(p.OrderBy("sequence")).Limit(p.Limit)

	case CursorTypeOffset:
		fallthrough
	default:
		return query.Order(p.OrderBy("sequence")).Offset(p.Offset).Limit(p.Limit)
	}
}

func (p *Pagination) ApplyToBlock(query *gorm.DB) *gorm.DB {
	switch p.CursorType {
	case CursorTypeHeight:
		height := int64(p.CursorValue["height"].(float64))
		if p.Order == OrderDesc {
			query = query.Where("height < ?", height)
		} else {
			query = query.Where("height > ?", height)
		}
		return query.Order(p.OrderBy("height")).Limit(p.Limit)

	case CursorTypeOffset:
		fallthrough
	default:
		return query.Order(p.OrderBy("height")).Offset(p.Offset).Limit(p.Limit)
	}
}

// Filtered Apply methods (with additional WHERE conditions)
func (p *Pagination) ApplyToEvmInternalTxWithFilter(query *gorm.DB) *gorm.DB {
	// Maintains existing filter conditions and adds cursor
	switch p.CursorType {
	case CursorTypeSequence:
		sequence := int64(p.CursorValue["sequence"].(float64))
		if p.Order == OrderDesc {
			query = query.Where("sequence < ?", sequence)
		} else {
			query = query.Where("sequence > ?", sequence)
		}
		return query.Order(p.OrderBy("sequence")).Limit(p.Limit)

	case CursorTypeOffset:
		fallthrough
	default:
		return query.Order(p.OrderBy("sequence")).Offset(p.Offset).Limit(p.Limit)
	}
}

func (p *Pagination) ApplyToTxWithFilter(query *gorm.DB) *gorm.DB {
	switch p.CursorType {
	case CursorTypeSequence:
		sequence := int64(p.CursorValue["sequence"].(float64))
		if p.Order == OrderDesc {
			query = query.Where("sequence < ?", sequence)
		} else {
			query = query.Where("sequence > ?", sequence)
		}
		return query.Order(p.OrderBy("sequence")).Limit(p.Limit)

	case CursorTypeOffset:
		fallthrough
	default:
		return query.Order(p.OrderBy("sequence")).Offset(p.Offset).Limit(p.Limit)
	}
}

func (p *Pagination) ApplyToEvmTxWithFilter(query *gorm.DB) *gorm.DB {
	switch p.CursorType {
	case CursorTypeSequence:
		sequence := int64(p.CursorValue["sequence"].(float64))
		if p.Order == OrderDesc {
			query = query.Where("sequence < ?", sequence)
		} else {
			query = query.Where("sequence > ?", sequence)
		}
		return query.Order(p.OrderBy("sequence")).Limit(p.Limit)

	case CursorTypeOffset:
		fallthrough
	default:
		return query.Order(p.OrderBy("sequence")).Offset(p.Offset).Limit(p.Limit)
	}
}
