// Package sessionnats wires cynork to NATS for session lifecycle (attached / activity / detached).
// See docs/tech_specs/nats_messaging.md and docs/dev_docs/_plan_005a_nats+pma_session_tracking.md Task 5.
package sessionnats

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
	"github.com/nats-io/nats.go"
)

const (
	defaultTenantID     = "default"
	producerServiceName = "cynode-cynork"
	clientTypeCynork    = "cynork"
)

// activityTickerInterval is the cadence for session.activity (overridable in tests via TestMain).
var activityTickerInterval = 2 * time.Minute

type natsDialFn func(cfg *natsutil.NatsConfig) (*nats.Conn, nats.JetStreamContext, error)

// Runtime holds a cynork NATS connection and session lifecycle publishers.
type Runtime struct {
	mu         sync.Mutex
	nc         *nats.Conn
	js         nats.JetStreamContext
	cancelLoop context.CancelFunc
	paused     atomic.Bool
	tenantID   string
	sessionID  string
	userID     string
	bindingKey string
}

// Start connects to NATS and starts session.attached plus a session.activity heartbeat when login includes a valid `nats` block.
// Returns (nil, nil) if NATS is not configured or credentials are incomplete.
func Start(ctx context.Context, log *slog.Logger, client *gateway.Client, login *userapi.LoginResponse) (*Runtime, error) {
	return startWithDial(ctx, log, client, login, natsutil.Connect)
}

func startWithDial(ctx context.Context, log *slog.Logger, client *gateway.Client, login *userapi.LoginResponse, dial natsDialFn) (*Runtime, error) {
	if login == nil || login.Nats == nil {
		return nil, nil
	}
	if login.InteractiveSessionID == "" || login.SessionBindingKey == "" {
		return nil, nil
	}
	cfg := &natsutil.NatsConfig{ClientCredentials: *login.Nats}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	me, err := client.GetMe(ctx)
	if err != nil {
		return nil, err
	}
	if dial == nil {
		dial = natsutil.Connect
	}
	nc, js, err := dial(cfg)
	if err != nil {
		return nil, err
	}
	r := &Runtime{
		nc:         nc,
		js:         js,
		tenantID:   defaultTenantID,
		sessionID:  login.InteractiveSessionID,
		userID:     me.ID,
		bindingKey: login.SessionBindingKey,
	}
	nc.SetDisconnectHandler(func(_ *nats.Conn) {
		r.paused.Store(true)
	})
	nc.SetReconnectHandler(func(_ *nats.Conn) {
		r.paused.Store(false)
	})
	loopCtx, cancelLoop := context.WithCancel(ctx)
	r.cancelLoop = cancelLoop
	if err := r.publishAttached(); err != nil {
		cancelLoop()
		_ = r.closeConn()
		return nil, err
	}
	go r.runActivityLoop(loopCtx, log)
	if log != nil {
		log.Info("cynork NATS session lifecycle started", "session_id", r.sessionID)
	}
	return r, nil
}

func (r *Runtime) runActivityLoop(ctx context.Context, log *slog.Logger) {
	ticker := time.NewTicker(activityTickerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if r.paused.Load() {
				continue
			}
			if err := r.publishActivity(); err != nil && log != nil {
				log.Debug("cynork NATS session activity", "error", err)
			}
		}
	}
}

func (r *Runtime) sessionPublishContext() (natsutil.Producer, natsutil.Scope, natsutil.Correlation, string) {
	now := time.Now().UTC().Format(time.RFC3339)
	prod := natsutil.Producer{Service: producerServiceName, InstanceID: r.sessionID}
	scope := natsutil.Scope{TenantID: r.tenantID, Sensitivity: "internal"}
	sid := r.sessionID
	corr := natsutil.Correlation{SessionID: &sid}
	return prod, scope, corr, now
}

type sessionPublishKind int

const (
	publishKindAttached sessionPublishKind = iota
	publishKindActivity
	publishKindDetached
)

func (r *Runtime) publishSessionLifecycle(kind sessionPublishKind, detachedReason string) error {
	r.mu.Lock()
	nc, js := r.nc, r.js
	r.mu.Unlock()
	if js == nil {
		return nil
	}
	prod, scope, corr, now := r.sessionPublishContext()
	switch kind {
	case publishKindAttached:
		payload := &natsutil.SessionAttachedPayloadV1{
			SessionID:  r.sessionID,
			UserID:     r.userID,
			BindingKey: r.bindingKey,
			ClientType: clientTypeCynork,
			Ts:         now,
		}
		return natsutil.PublishSessionAttached(nc, js, r.tenantID, r.sessionID, prod, scope, corr, payload)
	case publishKindActivity:
		payload := &natsutil.SessionActivityPayloadV1{
			SessionID:  r.sessionID,
			UserID:     r.userID,
			BindingKey: r.bindingKey,
			ClientType: clientTypeCynork,
			Ts:         now,
		}
		return natsutil.PublishSessionActivity(nc, js, r.tenantID, r.sessionID, prod, scope, corr, payload)
	case publishKindDetached:
		payload := &natsutil.SessionDetachedPayloadV1{
			SessionID:  r.sessionID,
			UserID:     r.userID,
			BindingKey: r.bindingKey,
			Reason:     detachedReason,
			Ts:         now,
		}
		return natsutil.PublishSessionDetached(nc, js, r.tenantID, r.sessionID, prod, scope, corr, payload)
	default:
		return nil
	}
}

func (r *Runtime) publishAttached() error {
	return r.publishSessionLifecycle(publishKindAttached, "")
}

func (r *Runtime) publishActivity() error {
	return r.publishSessionLifecycle(publishKindActivity, "")
}

func (r *Runtime) publishDetached(reason string) error {
	return r.publishSessionLifecycle(publishKindDetached, reason)
}

// Close stops the activity loop and closes the connection. If reason is non-empty, publishes session.detached first.
func (r *Runtime) Close(reason string) {
	if r == nil {
		return
	}
	if r.cancelLoop != nil {
		r.cancelLoop()
		r.cancelLoop = nil
	}
	if reason != "" {
		_ = r.publishDetached(reason)
	}
	_ = r.closeConn()
}

func (r *Runtime) closeConn() error {
	r.mu.Lock()
	nc := r.nc
	r.nc = nil
	r.js = nil
	r.mu.Unlock()
	if nc != nil {
		return natsutil.CloseConn(nc)
	}
	return nil
}
