// Package natsjwt issues NATS decentralized-auth user JWTs for session and worker clients.
// See docs/tech_specs/nats_messaging.md (CYNAI.USRGWY.NatsClientCredentials).
package natsjwt

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	jwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

// DefaultTenantID is used when no tenant is modeled (single-tenant dev).
const DefaultTenantID = "default"

// Issuer signs NATS user JWTs with the account signing key.
type Issuer struct {
	accountPub string
	signer     nkeys.KeyPair
	revoked    sync.Map // jti string -> struct{}
}

// NewIssuer parses account and signing seeds and builds an issuer.
func NewIssuer(accountSeed, signingSeed string) (*Issuer, error) {
	if strings.TrimSpace(accountSeed) == "" || strings.TrimSpace(signingSeed) == "" {
		return nil, errors.New("natsjwt: empty account or signing seed")
	}
	akp, err := nkeys.FromSeed([]byte(accountSeed))
	if err != nil {
		return nil, fmt.Errorf("natsjwt: account seed: %w", err)
	}
	accPub, err := akp.PublicKey()
	if err != nil {
		return nil, err
	}
	skp, err := nkeys.FromSeed([]byte(signingSeed))
	if err != nil {
		return nil, fmt.Errorf("natsjwt: signing seed: %w", err)
	}
	return &Issuer{accountPub: accPub, signer: skp}, nil
}

// NewDevIssuer returns the issuer using seeds from LoadDevSeeds (env, seeds file, or generated).
func NewDevIssuer() (*Issuer, error) {
	s, err := LoadDevSeeds()
	if err != nil {
		return nil, err
	}
	return NewIssuer(s.CynodeAccount, s.CynodeSigning)
}

// Revoked returns true if the JWT id was revoked (logout / admin revoke).
func (i *Issuer) Revoked(jti string) bool {
	if jti == "" {
		return false
	}
	_, ok := i.revoked.Load(jti)
	return ok
}

// RevokeJTI records a JWT id as revoked (best-effort; full NATS enforcement updates the account JWT).
func (i *Issuer) RevokeJTI(jti string) {
	if jti == "" {
		return
	}
	i.revoked.Store(jti, struct{}{})
}

// RevokeSessionNatsJWT decodes a user JWT and records its jti as revoked.
func (i *Issuer) RevokeSessionNatsJWT(token string) error {
	uc, err := jwt.DecodeUserClaims(token)
	if err != nil {
		return err
	}
	i.RevokeJTI(uc.ID)
	return nil
}

// SessionJWT returns a NATS user JWT for an interactive session (cynork / Web Console).
func (i *Issuer) SessionJWT(tenantID string, sessionID uuid.UUID, expires time.Time) (string, error) {
	if tenantID == "" {
		tenantID = DefaultTenantID
	}
	sid := sessionID.String()
	ukp, err := nkeys.CreateUser()
	if err != nil {
		return "", err
	}
	upub, err := ukp.PublicKey()
	if err != nil {
		return "", err
	}
	uc := jwt.NewUserClaims(upub)
	uc.IssuerAccount = i.accountPub
	uc.Expires = expires.Unix()
	uc.BearerToken = true
	// Phase 1 session lifecycle + chat (spec nats_messaging.md).
	uc.Pub.Allow.Add(
		fmt.Sprintf("cynode.session.activity.%s.%s", tenantID, sid),
		fmt.Sprintf("cynode.session.attached.%s.%s", tenantID, sid),
		fmt.Sprintf("cynode.session.detached.%s.%s", tenantID, sid),
		fmt.Sprintf("cynode.chat.request.%s", sid),
	)
	uc.Sub.Allow.Add(
		fmt.Sprintf("cynode.chat.stream.%s.>", sid),
		fmt.Sprintf("cynode.chat.amendment.%s.>", sid),
		fmt.Sprintf("cynode.chat.done.%s.>", sid),
		// Same-session activity subject: observe gateway- or self-published heartbeats (e.g. E2E, diagnostics).
		fmt.Sprintf("cynode.session.activity.%s.%s", tenantID, sid),
	)
	return uc.Encode(i.signer)
}

// NodeJWT returns a NATS user JWT for a worker node (system/worker role).
func (i *Issuer) NodeJWT(tenantID string, nodeID uuid.UUID, expires time.Time) (string, error) {
	if tenantID == "" {
		tenantID = DefaultTenantID
	}
	nid := nodeID.String()
	ukp, err := nkeys.CreateUser()
	if err != nil {
		return "", err
	}
	upub, err := ukp.PublicKey()
	if err != nil {
		return "", err
	}
	uc := jwt.NewUserClaims(upub)
	uc.IssuerAccount = i.accountPub
	uc.Expires = expires.Unix()
	uc.BearerToken = true
	uc.Pub.Allow.Add(
		"cynode.chat.stream.>",
		"cynode.chat.done.>",
		"cynode.session.activity.>",
		fmt.Sprintf("cynode.node.config_changed.%s.%s", tenantID, nid),
	)
	uc.Sub.Allow.Add(
		"cynode.chat.request.>",
		fmt.Sprintf("cynode.node.config_changed.%s.%s", tenantID, nid),
	)
	return uc.Encode(i.signer)
}

// ControlPlaneServiceJWT returns a NATS user JWT for the control-plane: subscribe to session lifecycle
// JetStream subjects and publish node.config_changed notifications.
func (i *Issuer) ControlPlaneServiceJWT(expires time.Time) (string, error) {
	ukp, err := nkeys.CreateUser()
	if err != nil {
		return "", err
	}
	upub, err := ukp.PublicKey()
	if err != nil {
		return "", err
	}
	uc := jwt.NewUserClaims(upub)
	uc.IssuerAccount = i.accountPub
	uc.Expires = expires.Unix()
	uc.BearerToken = true
	uc.Sub.Allow.Add(
		// Broad pattern matches subscribeCynodeSession ("cynode.session.>") on the control-plane.
		"cynode.session.>",
		// JetStream metadata API (EnsureStreams, consumers) uses request/reply on $JS.API.* + _INBOX.
		"_INBOX.>",
	)
	uc.Pub.Allow.Add(
		"cynode.node.config_changed.>",
		"$JS.API.>",
	)
	return uc.Encode(i.signer)
}

// GatewaySessionPublisherJWT returns a NATS user JWT for the user-gateway to publish session lifecycle
// messages (e.g. session.activity from REST API traffic, session.attached on login).
func (i *Issuer) GatewaySessionPublisherJWT(expires time.Time) (string, error) {
	ukp, err := nkeys.CreateUser()
	if err != nil {
		return "", err
	}
	upub, err := ukp.PublicKey()
	if err != nil {
		return "", err
	}
	uc := jwt.NewUserClaims(upub)
	uc.IssuerAccount = i.accountPub
	uc.Expires = expires.Unix()
	uc.BearerToken = true
	uc.Pub.Allow.Add(
		"cynode.session.activity.>",
		"cynode.session.attached.>",
		"cynode.session.detached.>",
	)
	return uc.Encode(i.signer)
}

// ExtractJTI returns the jti from a NATS user JWT.
func ExtractJTI(token string) (string, error) {
	uc, err := jwt.DecodeUserClaims(token)
	if err != nil {
		return "", err
	}
	return uc.ID, nil
}
