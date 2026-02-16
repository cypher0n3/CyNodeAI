// Package handlers provides HTTP handlers for the CyNodeAI API.
// See docs/tech_specs/go_rest_api_standards.md for implementation standards.
package handlers

import (
	"encoding/json"
	"net/http"
)

// ProblemDetails represents RFC 9457 Problem Details response.
// See docs/tech_specs/go_rest_api_standards.md#error-format-and-status-codes
type ProblemDetails struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// Common error types.
const (
	ErrTypeValidation     = "urn:cynodeai:error:validation"
	ErrTypeAuthentication = "urn:cynodeai:error:authentication"
	ErrTypeAuthorization  = "urn:cynodeai:error:authorization"
	ErrTypeNotFound       = "urn:cynodeai:error:not_found"
	ErrTypeRateLimit      = "urn:cynodeai:error:rate_limit"
	ErrTypeInternal       = "urn:cynodeai:error:internal"
)

// WriteError writes a Problem Details error response.
func WriteError(w http.ResponseWriter, status int, errType, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	problem := ProblemDetails{
		Type:   errType,
		Title:  title,
		Status: status,
		Detail: detail,
	}

	_ = json.NewEncoder(w).Encode(problem)
}

// WriteBadRequest writes a 400 error.
func WriteBadRequest(w http.ResponseWriter, detail string) {
	WriteError(w, http.StatusBadRequest, ErrTypeValidation, "Bad Request", detail)
}

// WriteUnauthorized writes a 401 error.
func WriteUnauthorized(w http.ResponseWriter, detail string) {
	WriteError(w, http.StatusUnauthorized, ErrTypeAuthentication, "Unauthorized", detail)
}

// WriteForbidden writes a 403 error.
func WriteForbidden(w http.ResponseWriter, detail string) {
	WriteError(w, http.StatusForbidden, ErrTypeAuthorization, "Forbidden", detail)
}

// WriteNotFound writes a 404 error.
func WriteNotFound(w http.ResponseWriter, detail string) {
	WriteError(w, http.StatusNotFound, ErrTypeNotFound, "Not Found", detail)
}

// WriteTooManyRequests writes a 429 error.
func WriteTooManyRequests(w http.ResponseWriter, detail string) {
	WriteError(w, http.StatusTooManyRequests, ErrTypeRateLimit, "Too Many Requests", detail)
}

// WriteInternalError writes a 500 error.
func WriteInternalError(w http.ResponseWriter, detail string) {
	WriteError(w, http.StatusInternalServerError, ErrTypeInternal, "Internal Server Error", detail)
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
