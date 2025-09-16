//nolint:dupl
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

	// Validate offset is not negative
	if offset < 0 {
		return nil, errors.New("pagination.offset cannot be negative")
	}

	pagination := &Pagination{
		Limit:      limit,
		Offset:     offset,
		Order:      getOrder(c),
		CursorType: CursorTypeOffset, // default to offset-based pagination
	}

	if key != "" {
		if err := parsePaginationKey(key, pagination); err != nil {
			return nil, err
		}
	}

	return pagination, nil
}

// parsePaginationKey parses the pagination key and updates pagination accordingly
func parsePaginationKey(key string, pagination *Pagination) error {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return errors.New("pagination.key must be a valid base64 encoded string")
	}

	// Try JSON cursor format first
	if strings.HasPrefix(string(bytes.TrimSpace(decoded)), "{") {
		return parseJSONCursor(decoded, pagination)
	}

	// Fallback to traditional integer approach
	return parseIntegerCursor(string(decoded), pagination)
}

// parseJSONCursor attempts to parse JSON cursor format
func parseJSONCursor(decoded []byte, pagination *Pagination) error {
	var cursorData map[string]any
	if json.Unmarshal(decoded, &cursorData) == nil {
		pagination.CursorValue = cursorData
		pagination.CursorType = detectCursorType(cursorData)
		return nil
	}

	// JSON parsing failed, try fallback to integer with appropriate error message
	parsedOffset, err := strconv.Atoi(string(decoded))
	if err != nil || parsedOffset < 0 {
		return errors.New("invalid pagination.key format")
	}

	pagination.Offset = parsedOffset
	return nil
}

// parseIntegerCursor parses traditional integer cursor format
func parseIntegerCursor(decodedStr string, pagination *Pagination) error {
	parsedOffset, err := strconv.Atoi(decodedStr)
	if err != nil || parsedOffset < 0 {
		return errors.New("pagination.key must decode to a nonnegative integer")
	}

	pagination.Offset = parsedOffset
	return nil
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

// safeGetInt64 safely extracts int64 value from cursor data with error handling
func (p *Pagination) safeGetInt64(field string) (int64, error) {
	value, exists := p.CursorValue[field]
	if !exists {
		return 0, fmt.Errorf("cursor field '%s' not found", field)
	}

	switch v := value.(type) {
	case int64:
		return v, nil
	case float64:
		// Check if float64 can be safely converted to int64
		if v != float64(int64(v)) {
			return 0, fmt.Errorf("cursor field '%s' contains non-integer value: %f", field, v)
		}
		return int64(v), nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("cursor field '%s' has unsupported type %T: %v", field, v, v)
	}
}

// safeGetString safely extracts string value from cursor data
func (p *Pagination) safeGetString(field string) (string, error) {
	value, exists := p.CursorValue[field]
	if !exists {
		return "", fmt.Errorf("cursor field '%s' not found", field)
	}

	if str, ok := value.(string); ok {
		return str, nil
	}
	return "", fmt.Errorf("cursor field '%s' is not a string: %T", field, value)
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
		sequence, err := p.safeGetInt64("sequence")
		if err != nil {
			// Fallback to offset-based pagination on error
			return query.Order(p.OrderBy("sequence")).Offset(p.Offset).Limit(p.Limit)
		}
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
		height, err := p.safeGetInt64("height")
		if err != nil {
			// Fallback to offset-based pagination on error
			return query.Order(p.OrderBy("height")).Offset(p.Offset).Limit(p.Limit)
		}
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

// getNftFallbackOrder returns the appropriate fallback ordering for NFT queries
func (p *Pagination) getNftFallbackOrder(query *gorm.DB, orderBy string) *gorm.DB {
	if orderBy == "token_id" {
		return query.Order(p.OrderBy("token_id", "height")).Offset(p.Offset).Limit(p.Limit)
	}
	return query.Order(p.OrderBy("height", "token_id")).Offset(p.Offset).Limit(p.Limit)
}

func (p *Pagination) ApplyToNft(query *gorm.DB, orderBy string) *gorm.DB {
	switch p.CursorType {
	case CursorTypeComposite:
		height, errHeight := p.safeGetInt64("height")
		tokenId, errTokenId := p.safeGetString("token_id")
		if errHeight != nil || errTokenId != nil {
			return p.getNftFallbackOrder(query, orderBy)
		}

		// The order of fields in WHERE and ORDER BY must match.
		if orderBy == "token_id" {
			if p.Order == OrderDesc {
				query = query.Where("(token_id, height) < (?, ?)", tokenId, height)
			} else {
				query = query.Where("(token_id, height) > (?, ?)", tokenId, height)
			}
			return query.Order(p.OrderBy("token_id", "height")).Limit(p.Limit)
		}

		// Default to ordering by height
		if p.Order == OrderDesc {
			query = query.Where("(height, token_id) < (?, ?)", height, tokenId)
		} else {
			query = query.Where("(height, token_id) > (?, ?)", height, tokenId)
		}
		return query.Order(p.OrderBy("height", "token_id")).Limit(p.Limit)

	case CursorTypeHeight: // Fallback for incomplete composite cursor
		height, err := p.safeGetInt64("height")
		if err != nil {
			return p.getNftFallbackOrder(query, orderBy)
		}
		if p.Order == OrderDesc {
			query = query.Where("height < ?", height)
		} else {
			query = query.Where("height > ?", height)
		}
		// Use the default fallback order to ensure consistent secondary sort
		return p.getNftFallbackOrder(query, orderBy)

	case CursorTypeOffset:
		fallthrough
	default:
		return p.getNftFallbackOrder(query, orderBy)
	}
}

// Additional Apply methods for other table types
func (p *Pagination) ApplyToTx(query *gorm.DB) *gorm.DB {
	switch p.CursorType {
	case CursorTypeSequence:
		sequence, err := p.safeGetInt64("sequence")
		if err != nil {
			// Fallback to offset-based pagination on error
			return query.Order(p.OrderBy("sequence")).Offset(p.Offset).Limit(p.Limit)
		}
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
		sequence, err := p.safeGetInt64("sequence")
		if err != nil {
			// Fallback to offset-based pagination on error
			return query.Order(p.OrderBy("sequence")).Offset(p.Offset).Limit(p.Limit)
		}
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
		height, err := p.safeGetInt64("height")
		if err != nil {
			// Fallback to offset-based pagination on error
			return query.Order(p.OrderBy("height")).Offset(p.Offset).Limit(p.Limit)
		}
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
		sequence, err := p.safeGetInt64("sequence")
		if err != nil {
			// Fallback to offset-based pagination on error
			return query.Order(p.OrderBy("sequence")).Offset(p.Offset).Limit(p.Limit)
		}
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
		sequence, err := p.safeGetInt64("sequence")
		if err != nil {
			// Fallback to offset-based pagination on error
			return query.Order(p.OrderBy("sequence")).Offset(p.Offset).Limit(p.Limit)
		}
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
		sequence, err := p.safeGetInt64("sequence")
		if err != nil {
			// Fallback to offset-based pagination on error
			return query.Order(p.OrderBy("sequence")).Offset(p.Offset).Limit(p.Limit)
		}
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
