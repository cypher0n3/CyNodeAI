package natsutil_test

import (
	"strings"
	"testing"
	"time"

	jwtlib "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nkeys"

	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
)

func TestConnect_NilConfig(t *testing.T) {
	t.Parallel()
	_, _, err := natsutil.Connect(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "nil config") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestConnect_NonBearerJWT(t *testing.T) {
	t.Parallel()
	akp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	ukp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	upub, err := ukp.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	uc := jwtlib.NewUserClaims(upub)
	uc.Expires = time.Now().Add(time.Hour).Unix()
	uc.BearerToken = false
	tok, err := uc.Encode(akp)
	if err != nil {
		t.Fatal(err)
	}
	cfg := natsutil.NatsConfig{}
	cfg.URL = testNATSURL
	cfg.JWT = tok
	cfg.JWTExpiresAt = time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	_, _, err = natsutil.Connect(&cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bearer") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestConnect_NatsDialFails(t *testing.T) {
	akp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	ukp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	upub, err := ukp.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	uc := jwtlib.NewUserClaims(upub)
	uc.Expires = time.Now().Add(time.Hour).Unix()
	uc.BearerToken = true
	tok, err := uc.Encode(akp)
	if err != nil {
		t.Fatal(err)
	}
	cfg := natsutil.NatsConfig{}
	cfg.URL = "nats://127.0.0.1:1"
	cfg.JWT = tok
	cfg.JWTExpiresAt = time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	_, _, err = natsutil.Connect(&cfg)
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestConnect_Success(t *testing.T) {
	opts := test.DefaultTestOptions
	opts.Port = -1
	s := test.RunServer(&opts)
	defer s.Shutdown()

	akp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	ukp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	upub, err := ukp.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	uc := jwtlib.NewUserClaims(upub)
	uc.Expires = time.Now().Add(time.Hour).Unix()
	uc.BearerToken = true
	tok, err := uc.Encode(akp)
	if err != nil {
		t.Fatal(err)
	}
	cfg := natsutil.NatsConfig{}
	cfg.URL = s.ClientURL()
	cfg.JWT = tok
	cfg.JWTExpiresAt = time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	nc, js, err := natsutil.Connect(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	if js == nil {
		t.Fatal("expected JetStream context")
	}
}

func TestConnect_InvalidJWT(t *testing.T) {
	t.Parallel()
	cfg := natsutil.NatsConfig{}
	cfg.URL = testNATSURL
	cfg.JWT = "not-a-jwt"
	cfg.JWTExpiresAt = time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	_, _, err := natsutil.Connect(&cfg)
	if err == nil {
		t.Fatal("expected error from invalid jwt")
	}
	if !strings.Contains(err.Error(), "natsutil: jwt") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnect_InvalidCA(t *testing.T) {
	t.Parallel()
	akp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	ukp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	upub, err := ukp.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	uc := jwtlib.NewUserClaims(upub)
	uc.Expires = time.Now().Add(time.Hour).Unix()
	uc.BearerToken = true
	tok, err := uc.Encode(akp)
	if err != nil {
		t.Fatal(err)
	}

	cfg := natsutil.NatsConfig{}
	cfg.URL = testNATSURL
	cfg.JWT = tok
	cfg.JWTExpiresAt = time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	cfg.CABundlePEM = "not pem"
	_, _, err = natsutil.Connect(&cfg)
	if err == nil {
		t.Fatal("expected error from bad ca bundle")
	}
	if !strings.Contains(err.Error(), "ca_bundle_pem") {
		t.Fatalf("unexpected error: %v", err)
	}
}
