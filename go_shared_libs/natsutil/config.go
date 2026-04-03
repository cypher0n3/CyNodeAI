// Package natsutil provides NATS connection helpers, JetStream setup, and session lifecycle publishers.
// See docs/tech_specs/nats_messaging.md.
package natsutil

import (
	"fmt"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/natsconfig"
)

// NatsConfig is the `nats` object from login or worker bootstrap (same JSON shape as [natsconfig.ClientCredentials]).
type NatsConfig struct {
	natsconfig.ClientCredentials
}

// Validate checks required fields for a NATS connection.
func (c *NatsConfig) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("natsutil: url is required")
	}
	if c.JWT == "" {
		return fmt.Errorf("natsutil: jwt is required")
	}
	if c.JWTExpiresAt == "" {
		return fmt.Errorf("natsutil: jwt_expires_at is required")
	}
	if _, err := time.Parse(time.RFC3339, c.JWTExpiresAt); err != nil {
		return fmt.Errorf("natsutil: jwt_expires_at: %w", err)
	}
	return nil
}
