package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestDevResetPMASessionStateHandler_Unauthorized(t *testing.T) {
	db := testutil.NewMockDB()
	h := DevResetPMASessionStateHandler(db, "correct-psk", nil)
	req := httptest.NewRequest(http.MethodPost, "/internal/dev/reset-pma-session-state", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestDevResetPMASessionStateHandler_DisabledWhenPSKEmpty(t *testing.T) {
	db := testutil.NewMockDB()
	h := DevResetPMASessionStateHandler(db, "", nil)
	req := httptest.NewRequest(http.MethodPost, "/internal/dev/reset-pma-session-state", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDevResetPMASessionStateHandler_MethodNotAllowed(t *testing.T) {
	db := testutil.NewMockDB()
	h := DevResetPMASessionStateHandler(db, "psk", nil)
	req := httptest.NewRequest(http.MethodGet, "/internal/dev/reset-pma-session-state", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code=%d want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestDevResetPMASessionStateHandler_OK(t *testing.T) {
	db := testutil.NewMockDB()
	h := DevResetPMASessionStateHandler(db, "psk", nil)
	req := httptest.NewRequest(http.MethodPost, "/internal/dev/reset-pma-session-state", strings.NewReader(""))
	req.Header.Set("Authorization", "Bearer psk")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}
