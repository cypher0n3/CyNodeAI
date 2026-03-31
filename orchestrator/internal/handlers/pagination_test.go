package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPagination is the plan-004 gate: shared list endpoints honor limit/offset and defaults.
func TestPagination(t *testing.T) {
	t.Run("valid limit and offset", func(t *testing.T) {
		cases := []struct {
			path       string
			wantLimit  int
			wantOffset int
		}{
			{"/", 50, 0},
			{"/?limit=10&offset=20", 10, 20},
		}
		for _, c := range cases {
			req := httptest.NewRequest("GET", c.path, http.NoBody)
			limit, offset, ok := parseLimitOffsetQuery(req, 50, 100)
			if !ok || limit != c.wantLimit || offset != c.wantOffset {
				t.Fatalf("path %q: got limit=%d offset=%d ok=%v", c.path, limit, offset, ok)
			}
		}
	})
	t.Run("reject limit above max", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?limit=101", http.NoBody)
		_, _, ok := parseLimitOffsetQuery(req, 50, 100)
		if ok {
			t.Fatal("expected invalid")
		}
	})
	t.Run("reject negative offset", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?offset=-1", http.NoBody)
		_, _, ok := parseLimitOffsetQuery(req, 50, 100)
		if ok {
			t.Fatal("expected invalid")
		}
	})
}
