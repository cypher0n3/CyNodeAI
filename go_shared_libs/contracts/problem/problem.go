// Package problem defines RFC 9457 Problem Details responses.
package problem

// Details represents an RFC 9457 Problem Details response.
// See docs/tech_specs/go_rest_api_standards.md#error-format-and-status-codes.
type Details struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

const (
	TypeValidation     = "urn:cynodeai:error:validation"
	TypeAuthentication = "urn:cynodeai:error:authentication"
	TypeAuthorization  = "urn:cynodeai:error:authorization"
	TypeNotFound       = "urn:cynodeai:error:not_found"
	TypeRateLimit      = "urn:cynodeai:error:rate_limit"
	TypeInternal       = "urn:cynodeai:error:internal"
)
