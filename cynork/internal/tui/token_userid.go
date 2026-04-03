package tui

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// userIDFromAccessTokenUnverified returns the JWT "sub" claim without verifying the signature.
// Used for TUI disk cache keys when GET /v1/users/me fails transiently but a bearer token is present.
func userIDFromAccessTokenUnverified(accessToken string) string {
	t := strings.TrimSpace(accessToken)
	if t == "" {
		return ""
	}
	parts := strings.Split(t, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	return strings.TrimSpace(claims.Sub)
}
