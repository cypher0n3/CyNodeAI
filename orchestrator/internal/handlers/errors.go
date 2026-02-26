// Package handlers provides HTTP handlers for the CyNodeAI API.
// See docs/tech_specs/go_rest_api_standards.md for implementation standards.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
)

// WriteError writes a Problem Details error response using shared problem.Details.
func WriteError(w http.ResponseWriter, status int, errType, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	p := problem.Details{
		Type:   errType,
		Title:  title,
		Status: status,
		Detail: detail,
	}
	_ = json.NewEncoder(w).Encode(p)
}

// WriteBadRequest writes a 400 error.
func WriteBadRequest(w http.ResponseWriter, detail string) {
	WriteError(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", detail)
}

// WriteUnauthorized writes a 401 error.
func WriteUnauthorized(w http.ResponseWriter, detail string) {
	WriteError(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", detail)
}

// WriteForbidden writes a 403 error.
func WriteForbidden(w http.ResponseWriter, detail string) {
	WriteError(w, http.StatusForbidden, problem.TypeAuthorization, "Forbidden", detail)
}

// WriteNotFound writes a 404 error.
func WriteNotFound(w http.ResponseWriter, detail string) {
	WriteError(w, http.StatusNotFound, problem.TypeNotFound, "Not Found", detail)
}

// WriteTooManyRequests writes a 429 error.
func WriteTooManyRequests(w http.ResponseWriter, detail string) {
	WriteError(w, http.StatusTooManyRequests, problem.TypeRateLimit, "Too Many Requests", detail)
}

// WriteInternalError writes a 500 error.
func WriteInternalError(w http.ResponseWriter, detail string) {
	WriteError(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", detail)
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
