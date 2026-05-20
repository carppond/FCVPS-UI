package util

import (
	"net/http"
	"strconv"

	"shiguang-vps/internal/config"
)

// Pagination is the parsed `page` + `page_size` query state.
type Pagination struct {
	Page     int
	PageSize int
}

// Offset returns the SQL OFFSET value for the current page (zero-based).
func (p Pagination) Offset() int {
	if p.Page <= 1 {
		return 0
	}
	return (p.Page - 1) * p.PageSize
}

// Limit returns the SQL LIMIT value (same as PageSize but reads cleaner).
func (p Pagination) Limit() int { return p.PageSize }

// ParsePaginationQuery extracts `page` and `page_size` from r.URL query.
// Invalid / missing values fall back to the project defaults; page_size is
// clamped to MaxPaginationPageSize.
func ParsePaginationQuery(r *http.Request) Pagination {
	if r == nil {
		return Pagination{
			Page:     config.DefaultPaginationPage,
			PageSize: config.DefaultPaginationPageSize,
		}
	}
	q := r.URL.Query()
	return Pagination{
		Page:     parsePositiveInt(q.Get("page"), config.DefaultPaginationPage),
		PageSize: clampPageSize(parsePositiveInt(q.Get("page_size"), config.DefaultPaginationPageSize)),
	}
}

// parsePositiveInt returns the integer in s if it parses and is > 0,
// otherwise fallback.
func parsePositiveInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// clampPageSize forces page_size into [1, MaxPaginationPageSize].
func clampPageSize(n int) int {
	if n <= 0 {
		return config.DefaultPaginationPageSize
	}
	if n > config.MaxPaginationPageSize {
		return config.MaxPaginationPageSize
	}
	return n
}
