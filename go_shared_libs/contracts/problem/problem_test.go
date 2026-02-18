package problem

import (
	"testing"
)

func TestDetails_Validate(t *testing.T) {
	tests := []struct {
		name    string
		d       *Details
		wantErr bool
	}{
		{"nil is valid", nil, false},
		{"4xx is valid", &Details{Status: 400}, false},
		{"5xx is valid", &Details{Status: 500}, false},
		{"499 is valid", &Details{Status: 499}, false},
		{"2xx is invalid", &Details{Status: 200}, true},
		{"3xx is invalid", &Details{Status: 302}, true},
		{"399 is invalid", &Details{Status: 399}, true},
		{"600 is invalid", &Details{Status: 600}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.d.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	e := &ValidationError{Status: 200}
	if e.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

func TestTypeConstants(t *testing.T) {
	// Ensure type constants are present (compile-time check; no executable stmts in const block).
	_ = TypeValidation
	_ = TypeAuthentication
	_ = TypeAuthorization
	_ = TypeNotFound
	_ = TypeRateLimit
	_ = TypeInternal
}
