package handlers

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/natsjwt"
	"github.com/nats-io/nats.go"
)

const gatewaySessionProducer = "cynode-gateway"

// GatewaySessionPublisher publishes session lifecycle events from the user-gateway (JetStream), including
// session.activity derived from REST API traffic when clients are not publishing from their own NATS connection.
type GatewaySessionPublisher struct {
	nc *nats.Conn
	js nats.JetStreamContext
}

// NewGatewaySessionPublisher returns a publisher, or nil if js is nil.
func NewGatewaySessionPublisher(nc *nats.Conn, js nats.JetStreamContext) *GatewaySessionPublisher {
	if js == nil {
		return nil
	}
	return &GatewaySessionPublisher{nc: nc, js: js}
}

// PublishAttached publishes session.attached for the interactive session.
func (g *GatewaySessionPublisher) PublishAttached(_ context.Context, tenantID, sessionID, userID, bindingKey string) error {
	if g == nil || g.js == nil {
		return nil
	}
	prod := natsutil.Producer{Service: gatewaySessionProducer, InstanceID: sessionID}
	scope := natsutil.Scope{TenantID: tenantID, Sensitivity: "internal"}
	sid := sessionID
	corr := natsutil.Correlation{SessionID: &sid}
	now := time.Now().UTC().Format(time.RFC3339)
	payload := &natsutil.SessionAttachedPayloadV1{
		SessionID:  sessionID,
		UserID:     userID,
		BindingKey: bindingKey,
		ClientType: "http",
		Ts:         now,
	}
	tid := tenantID
	if tid == "" {
		tid = natsjwt.DefaultTenantID
	}
	return natsutil.PublishSessionAttached(g.nc, g.js, tid, sessionID, prod, scope, corr, payload)
}

// PublishActivity publishes session.activity for the latest active binding for this user (gateway API liveness).
// Matches TouchPMABindingActivity / PMA routing so stale session bindings are not kept alive via NATS touches.
func (g *GatewaySessionPublisher) PublishActivity(ctx context.Context, db database.SessionBindingStore, tenantID string, userID uuid.UUID) error {
	if g == nil || g.js == nil || db == nil {
		return nil
	}
	bindings, err := db.ListActiveBindingsForUser(ctx, userID)
	if err != nil {
		return err
	}
	b := pickLatestSessionBinding(bindings)
	if b == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	prod := natsutil.Producer{Service: gatewaySessionProducer, InstanceID: userID.String()}
	scope := natsutil.Scope{TenantID: tenantID, Sensitivity: "internal"}
	tid := tenantID
	if tid == "" {
		tid = natsjwt.DefaultTenantID
	}
	uidStr := userID.String()
	sid := b.SessionID.String()
	corr := natsutil.Correlation{SessionID: &sid}
	payload := &natsutil.SessionActivityPayloadV1{
		SessionID:  sid,
		UserID:     uidStr,
		BindingKey: b.BindingKey,
		ClientType: "http",
		Ts:         now,
	}
	return natsutil.PublishSessionActivity(g.nc, g.js, tid, sid, prod, scope, corr, payload)
}

// PublishDetached publishes session.detached with the given reason.
func (g *GatewaySessionPublisher) PublishDetached(_ context.Context, tenantID, sessionID, userID, bindingKey, reason string) error {
	if g == nil || g.js == nil {
		return nil
	}
	prod := natsutil.Producer{Service: gatewaySessionProducer, InstanceID: sessionID}
	scope := natsutil.Scope{TenantID: tenantID, Sensitivity: "internal"}
	sid := sessionID
	corr := natsutil.Correlation{SessionID: &sid}
	now := time.Now().UTC().Format(time.RFC3339)
	payload := &natsutil.SessionDetachedPayloadV1{
		SessionID:  sessionID,
		UserID:     userID,
		BindingKey: bindingKey,
		Reason:     reason,
		Ts:         now,
	}
	tid := tenantID
	if tid == "" {
		tid = natsjwt.DefaultTenantID
	}
	return natsutil.PublishSessionDetached(g.nc, g.js, tid, sessionID, prod, scope, corr, payload)
}
