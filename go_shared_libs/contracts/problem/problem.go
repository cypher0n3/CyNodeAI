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

// Validate returns an error if Details has an invalid status (non-HTTP error range).
func (d *Details) Validate() error {
	if d == nil {
		return nil
	}
	if d.Status < 400 || d.Status >= 600 {
		return &ValidationError{Status: d.Status}
	}
	return nil
}

// ValidationError is returned when Details has an invalid status code.
type ValidationError struct {
	Status int
}

func (e *ValidationError) Error() string {
	return "problem details status must be 4xx or 5xx"
}
