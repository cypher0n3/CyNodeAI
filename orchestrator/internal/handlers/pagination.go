// Package handlers: shared query pagination helpers for list endpoints.
package handlers

import (
	"net/http"
	"strconv"
	"strings"
)

// parseLimitOffsetQuery parses limit and offset from r's query string.
// When the limit query parameter is absent, defaultLimit is used.
// When limit is present but invalid or out of range, ok is false.
// offset defaults to 0; negative values are treated as 0.
func parseLimitOffsetQuery(r *http.Request, defaultLimit, maxLimit int) (limit, offset int, ok bool) {
	limit = defaultLimit
	if l := strings.TrimSpace(r.URL.Query().Get("limit")); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n < 1 || n > maxLimit {
			return 0, 0, false
		}
		limit = n
	}
	offset = 0
	if o := strings.TrimSpace(r.URL.Query().Get("offset")); o != "" {
		n, err := strconv.Atoi(o)
		if err != nil || n < 0 {
			return 0, 0, false
		}
		offset = n
	}
	return limit, offset, true
}
