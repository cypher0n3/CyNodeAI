package natsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	jwtlib "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
)

// Connect establishes a TCP connection to NATS using the orchestrator-issued user JWT (bearer token),
// optional TLS from CA bundle, and returns a JetStream context.
func Connect(cfg *NatsConfig) (*nats.Conn, nats.JetStreamContext, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("natsutil: nil config")
	}
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	opts, err := connectOptions(cfg)
	if err != nil {
		return nil, nil, err
	}
	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		_ = nc.Drain()
		return nil, nil, err
	}
	return nc, js, nil
}

func connectOptions(cfg *NatsConfig) ([]nats.Option, error) {
	opts := []nats.Option{
		nats.Name("cynodeai-natsutil"),
		nats.Timeout(15 * time.Second),
		nats.MaxReconnects(60),
		nats.ReconnectWait(time.Second),
	}
	if cfg.CABundlePEM != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(cfg.CABundlePEM)) {
			return nil, fmt.Errorf("natsutil: ca_bundle_pem: no certificates parsed")
		}
		opts = append(opts, nats.Secure(&tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    pool,
		}))
	}
	uc, err := jwtlib.DecodeUserClaims(cfg.JWT)
	if err != nil {
		return nil, fmt.Errorf("natsutil: jwt: %w", err)
	}
	if !uc.IsBearerToken() {
		return nil, fmt.Errorf("natsutil: user jwt must be a bearer token for this client")
	}
	jwtStr := cfg.JWT
	opts = append(opts, nats.UserJWT(
		func() (string, error) { return jwtStr, nil },
		func(nonce []byte) ([]byte, error) {
			_ = nonce
			return []byte{}, nil
		},
	))
	return opts, nil
}
