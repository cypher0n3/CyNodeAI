// Command e2e-nats-subscribe-once waits for one message on a subject using natsutil (same JWT path as cynork).
// Used by Python E2E (e2e_0840). Env: NATS_URL, NATS_JWT, NATS_JWT_EXPIRES_AT (RFC3339), NATS_SUBJECT.
// Writes the raw message payload to stdout; logs to stderr. Exit 1 on timeout or connect error.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
)

func main() {
	os.Exit(run())
}

//nolint:gocyclo // Env validation, connect, subscribe, and one-shot read are sequential CLI steps.
func run() int {
	url := os.Getenv("NATS_URL")
	jwt := os.Getenv("NATS_JWT")
	exp := os.Getenv("NATS_JWT_EXPIRES_AT")
	subj := os.Getenv("NATS_SUBJECT")
	if url == "" || jwt == "" || exp == "" || subj == "" {
		fmt.Fprintln(os.Stderr, "e2e-nats-subscribe-once: set NATS_URL, NATS_JWT, NATS_JWT_EXPIRES_AT, NATS_SUBJECT")
		return 2
	}
	cfg := &natsutil.NatsConfig{}
	cfg.URL = url
	cfg.JWT = jwt
	cfg.JWTExpiresAt = exp
	if pem := os.Getenv("NATS_CA_BUNDLE_PEM"); pem != "" {
		cfg.CABundlePEM = pem
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	nc, _, err := natsutil.Connect(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = nc.Drain() }()

	// Core subscribe: gateway mirrors JetStream session.* publishes on the same subject for session JWTs.
	sub, err := nc.SubscribeSync(subj)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = sub.Unsubscribe() }()
	fmt.Fprintln(os.Stderr, "E2E_NATS_SUB_READY")

	timeout := 120 * time.Second
	if s := os.Getenv("NATS_WAIT_TIMEOUT"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			timeout = d
		}
	}
	msg, err := sub.NextMsg(timeout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "timeout waiting for message")
		return 1
	}
	if msg == nil || len(msg.Data) == 0 {
		fmt.Fprintln(os.Stderr, "empty message")
		return 1
	}
	if _, err := os.Stdout.Write(msg.Data); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
