package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/natsjwt"
)

const (
	evSessionActivity = "session.activity"
	evSessionAttached = "session.attached"
	evSessionDetached = "session.detached"
)

// RunSessionActivityConsumer subscribes to session lifecycle subjects and updates PMA bindings.
// Call from control-plane when NATS is available; blocks until ctx is canceled.
func RunSessionActivityConsumer(ctx context.Context, db database.Store, nc *nats.Conn, logger *slog.Logger) {
	if nc == nil || db == nil {
		return
	}
	sub, err := subscribeCynodeSession(nc, ctx, db, logger)
	if err != nil {
		if logger != nil {
			logger.Warn("nats session consumer", "error", err)
		}
		return
	}
	defer func() { _ = sub.Unsubscribe() }()
	if logger != nil {
		logger.Info("subscribed to NATS session lifecycle", "subject", "cynode.session.>")
	}
	<-ctx.Done()
}

func subscribeCynodeSession(nc *nats.Conn, ctx context.Context, db database.Store, logger *slog.Logger) (*nats.Subscription, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}
	if err := natsutil.EnsureStreams(js); err != nil {
		return nil, err
	}
	return nc.Subscribe("cynode.session.>", func(msg *nats.Msg) {
		if msg == nil || len(msg.Data) == 0 {
			return
		}
		if err := HandleSessionActivityMessage(ctx, db, msg.Data, logger); err != nil && logger != nil {
			logger.Debug("session activity nats handler", "error", err)
		}
	})
}

// HandleSessionActivityMessage processes one JSON envelope from NATS (tests and consumer).
func HandleSessionActivityMessage(ctx context.Context, db database.Store, data []byte, logger *slog.Logger) error {
	var env natsutil.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	switch env.EventType {
	case evSessionActivity:
		return handleSessionActivityEnvelope(ctx, db, &env, logger)
	case evSessionAttached:
		return handleSessionAttachedEnvelope(ctx, db, &env, logger)
	case evSessionDetached:
		return handleSessionDetachedEnvelope(ctx, db, &env, logger)
	default:
		return nil
	}
}

func payloadString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func handleSessionActivityEnvelope(ctx context.Context, db database.Store, env *natsutil.Envelope, _ *slog.Logger) error {
	bk := payloadString(env.Payload, "binding_key")
	if bk == "" {
		return nil
	}
	return db.TouchSessionBindingByKey(ctx, bk, time.Now().UTC())
}

func handleSessionAttachedEnvelope(ctx context.Context, db database.Store, env *natsutil.Envelope, logger *slog.Logger) error {
	bk := payloadString(env.Payload, "binding_key")
	if bk == "" {
		return nil
	}
	b, err := db.GetSessionBindingByKey(ctx, bk)
	if err != nil || b == nil {
		return err
	}
	if b.State != models.SessionBindingStateTeardownPending {
		return nil
	}
	return GreedyProvisionPMAAfterInteractiveSession(ctx, db, b.UserID, b.SessionID, logger)
}

func handleSessionDetachedEnvelope(ctx context.Context, db database.Store, env *natsutil.Envelope, logger *slog.Logger) error {
	reason := payloadString(env.Payload, "reason")
	sidStr := payloadString(env.Payload, "session_id")
	uidStr := payloadString(env.Payload, "user_id")
	if reason != "logout" || sidStr == "" || uidStr == "" {
		return nil
	}
	sid, err := uuid.Parse(sidStr)
	if err != nil {
		return err
	}
	uid, err := uuid.Parse(uidStr)
	if err != nil {
		return err
	}
	return TeardownPMAForInteractiveSession(ctx, db, uid, sid, reason, logger)
}

func issueNATSWithServiceJWT(url string, iss *natsjwt.Issuer, token func(time.Time) (string, error), ensureStreams bool) (*nats.Conn, nats.JetStreamContext, error) {
	if iss == nil || strings.TrimSpace(url) == "" {
		return nil, nil, nil
	}
	exp := time.Now().UTC().Add(24 * time.Hour)
	tok, err := token(exp)
	if err != nil {
		return nil, nil, err
	}
	cfg := &natsutil.NatsConfig{}
	cfg.URL = url
	cfg.JWT = tok
	cfg.JWTExpiresAt = exp.Format(time.RFC3339)
	nc, js, err := natsutil.Connect(cfg)
	if err != nil {
		return nil, nil, err
	}
	if ensureStreams {
		if err := natsutil.EnsureStreams(js); err != nil {
			_ = natsutil.CloseConn(nc)
			return nil, nil, err
		}
	}
	return nc, js, nil
}

// IssueControlPlaneNATSConnection builds a JWT and connects for the control-plane consumer + config bump publisher.
func IssueControlPlaneNATSConnection(url string, iss *natsjwt.Issuer) (*nats.Conn, nats.JetStreamContext, error) {
	return issueNATSWithServiceJWT(url, iss, iss.ControlPlaneServiceJWT, true)
}

// IssueGatewayNATSConnection connects the user-gateway for HTTP-only session publishing.
func IssueGatewayNATSConnection(url string, iss *natsjwt.Issuer) (*nats.Conn, nats.JetStreamContext, error) {
	return issueNATSWithServiceJWT(url, iss, iss.GatewaySessionPublisherJWT, false)
}
