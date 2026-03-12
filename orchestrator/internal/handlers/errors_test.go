package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
)

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	WriteError(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "test detail")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusBadRequest)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %s, want application/problem+json", ct)
	}

	var p problem.Details
	if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}

	if p.Type != problem.TypeValidation {
		t.Errorf("problem.Type = %s, want %s", p.Type, problem.TypeValidation)
	}

	if p.Status != http.StatusBadRequest {
		t.Errorf("problem.Status = %d, want %d", p.Status, http.StatusBadRequest)
	}

	if p.Detail != "test detail" {
		t.Errorf("problem.Detail = %s, want test detail", p.Detail)
	}
}

func TestWriteBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	WriteBadRequest(w, "bad request detail")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWriteUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	WriteUnauthorized(w, "unauthorized detail")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestWriteForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	WriteForbidden(w, "forbidden detail")

	if w.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestWriteNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	WriteNotFound(w, "not found detail")

	if w.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestWriteTooManyRequests(t *testing.T) {
	w := httptest.NewRecorder()
	WriteTooManyRequests(w, "rate limit detail")

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

func TestWriteInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteInternalError(w, "internal error detail")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"key": "value"}
	WriteJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", ct)
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("result[key] = %s, want value", result["key"])
	}
}
