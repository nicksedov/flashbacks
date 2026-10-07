package helpers

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// PaginationMode determines how page size is validated.
type PaginationMode int

const (
	// ModeFlexible validates 1 <= pageSize <= MaxPageSizeFlexible, defaults to DefaultPageSize.
	ModeFlexible PaginationMode = iota
	// ModeFixed validates against FixedPageSizes list, defaults to first value.
	ModeFixed
)

// PaginationParams holds parsed pagination parameters.
type PaginationParams struct {
	Page     int
	PageSize int
	Offset   int
}

// PaginationResult holds calculated pagination metadata.
type PaginationResult struct {
	Page        int
	PageSize    int
	TotalPages  int
	HasNextPage bool
	HasPrevPage bool
}

// ParsePagination parses page and pageSize from query parameters.
func ParsePagination(c *gin.Context, mode PaginationMode) PaginationParams {
	page := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "50")

	p, _ := parseInt(page)
	ps, _ := parseInt(pageSizeStr)

	if p < 1 {
		p = 1
	}

	switch mode {
	case ModeFixed:
		isValid := false
		for _, valid := range FixedPageSizes {
			if ps == valid {
				isValid = true
				break
			}
		}
		if !isValid {
			ps = FixedPageSizes[0]
		}
	case ModeFlexible:
		if ps < 1 || ps > MaxPageSizeFlexible {
			ps = DefaultPageSize
		}
	}

	return PaginationParams{
		Page:     p,
		PageSize: ps,
		Offset:   (p - 1) * ps,
	}
}

// CalcPagination calculates pagination metadata from total record count.
func CalcPagination(page, pageSize int, total int64) PaginationResult {
	tp := int(total)
	if tp > 0 {
		tp = (int(total) + pageSize - 1) / pageSize
	}
	if tp < 1 {
		tp = 1
	}
	if page > tp {
		page = tp
	}
	if page < 1 {
		page = 1
	}

	return PaginationResult{
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  tp,
		HasNextPage: page < tp,
		HasPrevPage: page > 1,
	}
}

func parseInt(s string) (int, bool) {
	result := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		result = result*10 + int(ch-'0')
	}
	return result, true
}

// CursorParams holds parsed cursor-based pagination parameters.
type CursorParams struct {
	Cursor    string
	Limit     int
	Direction string // "forward" or "backward"
}

// CursorResult holds cursor-based pagination metadata.
type CursorResult struct {
	NextCursor *string
	HasMore    bool
}

// EncodeCursor creates a cursor from a date string and image ID.
// Format: base64("YYYY-MM-DD HH:MM:SS|imageID")
func EncodeCursor(dateStr string, imageID uint) string {
	raw := fmt.Sprintf("%s|%06d", dateStr, imageID)
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor decodes a cursor into its date string and image ID components.
// Returns error if cursor is malformed.
// Supports both old format (YYYY-MM-DD|id) and new format (YYYY-MM-DD HH:MM:SS|id)
func DecodeCursor(cursor string) (string, uint, error) {
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", 0, fmt.Errorf("invalid cursor encoding: %w", err)
	}

	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid cursor format: expected 'date|id', got '%s'", string(decoded))
	}

	dateStr := parts[0]
	idStr := parts[1]

	imageID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return "", 0, fmt.Errorf("invalid image ID in cursor: %w", err)
	}

	return dateStr, uint(imageID), nil
}

// ParseCursorPagination parses cursor, limit, and direction from query parameters.
// Falls back to page-based pagination if cursor is not provided.
func ParseCursorPagination(c *gin.Context, defaultLimit int) (CursorParams, bool) {
	cursor := c.Query("cursor")

	// If no cursor, fall back to page-based pagination
	if cursor == "" {
		return CursorParams{}, false
	}

	limitStr := c.DefaultQuery("limit", strconv.Itoa(defaultLimit))
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = defaultLimit
	}
	if limit > 200 {
		limit = 200
	}

	direction := c.DefaultQuery("direction", "forward")
	if direction != "forward" && direction != "backward" {
		direction = "forward"
	}

	return CursorParams{
		Cursor:    cursor,
		Limit:     limit,
		Direction: direction,
	}, true
}

// CalcCursorPagination determines the next cursor from the last item in the result set.
// If resultCount > limit, the last item is used to build the next cursor.
func CalcCursorPagination(items []struct {
	Date string
	ID   uint
}, limit int) CursorResult {
	if len(items) <= limit {
		return CursorResult{
			NextCursor: nil,
			HasMore:    false,
		}
	}

	// Use the last item (at index limit) to build cursor
	lastItem := items[limit]
	nextCursor := EncodeCursor(lastItem.Date, lastItem.ID)

	return CursorResult{
		NextCursor: &nextCursor,
		HasMore:    true,
	}
}
