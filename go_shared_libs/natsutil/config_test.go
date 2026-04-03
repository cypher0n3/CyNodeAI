package natsutil_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
)

func TestNatsConfig_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	raw := fmt.Sprintf(`{
		"url": %q,
		"jwt": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.sig",
		"jwt_expires_at": "2026-12-31T23:59:59Z",
		"websocket_url": "ws://127.0.0.1:8223/nats",
		"ca_bundle_pem": "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n",
		"subjects": {"session_activity": "cynode.session.activity.default.s1"}
	}`, testNATSURL)
	var cfg natsutil.NatsConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.URL != testNATSURL {
		t.Fatalf("url: %q", cfg.URL)
	}
	if cfg.Subjects == nil || cfg.Subjects["session_activity"] != "cynode.session.activity.default.s1" {
		t.Fatalf("subjects: %#v", cfg.Subjects)
	}
	out, err := json.Marshal(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	var again natsutil.NatsConfig
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatal(err)
	}
	if again.JWTExpiresAt != cfg.JWTExpiresAt {
		t.Fatal("jwt_expires_at round trip")
	}
}

func TestNatsConfig_Validate(t *testing.T) {
	t.Parallel()
	exp := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)

	if err := (&natsutil.NatsConfig{}).Validate(); err == nil {
		t.Fatal("expected error for empty config")
	}

	noURL := natsutil.NatsConfig{}
	noURL.JWT = "x"
	noURL.JWTExpiresAt = exp
	if err := noURL.Validate(); err == nil {
		t.Fatal("expected error for missing url")
	}

	noJWT := natsutil.NatsConfig{}
	noJWT.URL = testNATSURL
	noJWT.JWTExpiresAt = exp
	if err := noJWT.Validate(); err == nil {
		t.Fatal("expected error for missing jwt")
	}

	cfg := natsutil.NatsConfig{}
	cfg.URL = testNATSURL
	cfg.JWT = "x"
	cfg.JWTExpiresAt = exp
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}

	cfg.JWTExpiresAt = "not-a-date"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected bad date error")
	}

	cfg = natsutil.NatsConfig{}
	cfg.URL = testNATSURL
	cfg.JWT = "x"
	cfg.JWTExpiresAt = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty jwt_expires_at")
	}
}
