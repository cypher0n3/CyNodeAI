package nodeagent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

const (
	defaultTenantID          = "default"
	sessionActivityInterval  = 2 * time.Minute
	natsJWTRefreshLead       = 10 * time.Minute
	natsRefreshCheckInterval = 1 * time.Minute
)

// NatsRuntime holds a worker NATS connection for session activity relay and config notifications.
type NatsRuntime struct {
	mu           sync.Mutex
	nc           *nats.Conn
	js           nats.JetStreamContext
	cfg          *natsutil.NatsConfig
	nodeID       uuid.UUID
	slug         string
	reconnectSig chan struct{}
}

// NewNatsRuntime connects to NATS when bootstrap includes a nats block; returns (nil, nil) if absent.
func NewNatsRuntime(_ context.Context, logger *slog.Logger, bootstrap *BootstrapData, nodeID uuid.UUID, nodeSlug string) (*NatsRuntime, error) {
	if bootstrap == nil || bootstrap.Nats == nil {
		return nil, nil
	}
	ncfg := &natsutil.NatsConfig{ClientCredentials: *bootstrap.Nats}
	if err := ncfg.Validate(); err != nil {
		return nil, err
	}
	nc, js, err := natsutil.Connect(ncfg)
	if err != nil {
		return nil, err
	}
	if logger != nil {
		logger.Info("connected to NATS for worker session relay")
	}
	return &NatsRuntime{
		nc:           nc,
		js:           js,
		cfg:          ncfg,
		nodeID:       nodeID,
		slug:         strings.TrimSpace(nodeSlug),
		reconnectSig: make(chan struct{}, 1),
	}, nil
}

// Close drains the NATS connection.
func (r *NatsRuntime) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	nc := r.nc
	r.nc = nil
	r.js = nil
	r.mu.Unlock()
	if nc != nil {
		_ = natsutil.CloseConn(nc)
	}
}

// RunSessionActivityLoop publishes session.activity for session-bound PMA rows until ctx is canceled.
func (r *NatsRuntime) RunSessionActivityLoop(ctx context.Context, logger *slog.Logger, getConfig func() *nodepayloads.NodeConfigurationPayload) {
	if r == nil {
		return
	}
	ticker := time.NewTicker(sessionActivityInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cfg := getConfig()
			if cfg == nil {
				continue
			}
			if err := r.publishSessionActivity(cfg); err != nil && logger != nil {
				logger.Debug("nats session activity publish", "error", err)
			}
		}
	}
}

func (r *NatsRuntime) publishSessionActivity(nodeConfig *nodepayloads.NodeConfigurationPayload) error {
	r.mu.Lock()
	nc, js := r.nc, r.js
	r.mu.Unlock()
	if js == nil || nodeConfig == nil || nodeConfig.ManagedServices == nil {
		return nil
	}
	prod := natsutil.Producer{Service: "cynode-worker", InstanceID: r.instanceID()}
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range nodeConfig.ManagedServices.Services {
		svc := &nodeConfig.ManagedServices.Services[i]
		if !strings.EqualFold(strings.TrimSpace(svc.ServiceType), serviceTypePMA) {
			continue
		}
		if svc.Env == nil {
			continue
		}
		sid := strings.TrimSpace(svc.Env["CYNODE_SESSION_ID"])
		uid := strings.TrimSpace(svc.Env["CYNODE_USER_ID"])
		tenant := strings.TrimSpace(svc.Env["CYNODE_TENANT_ID"])
		bk := strings.TrimSpace(svc.Env["CYNODE_BINDING_KEY"])
		if sid == "" || uid == "" || bk == "" {
			continue
		}
		if tenant == "" {
			tenant = defaultTenantID
		}
		scope := natsutil.Scope{TenantID: tenant, Sensitivity: "internal"}
		sidCopy := sid
		corr := natsutil.Correlation{SessionID: &sidCopy}
		payload := &natsutil.SessionActivityPayloadV1{
			SessionID:  sid,
			UserID:     uid,
			BindingKey: bk,
			ClientType: "other",
			Ts:         now,
		}
		if err := natsutil.PublishSessionActivity(nc, js, tenant, sid, prod, scope, corr, payload); err != nil {
			return err
		}
	}
	return nil
}

func (r *NatsRuntime) instanceID() string {
	if r.slug != "" {
		return r.slug
	}
	return r.nodeID.String()
}

// StartConfigSubscriber subscribes to node config_changed and signals bump (non-blocking) on each message.
// Resubscribes after JWT refresh reconnect.
//
//nolint:gocognit // resubscribe loop after JWT-driven reconnect
func (r *NatsRuntime) StartConfigSubscriber(ctx context.Context, logger *slog.Logger, bump chan<- struct{}) {
	if r == nil || bump == nil {
		return
	}
	subj := fmt.Sprintf("cynode.node.config_changed.%s.%s", defaultTenantID, r.nodeID.String())
	go func() {
		for {
			r.mu.Lock()
			nc := r.nc
			sig := r.reconnectSig
			r.mu.Unlock()
			if nc == nil {
				return
			}
			sub, err := nc.Subscribe(subj, func(_ *nats.Msg) {
				select {
				case bump <- struct{}{}:
				default:
				}
			})
			if err != nil {
				if logger != nil {
					logger.Warn("nats config subscriber", "error", err, "subject", subj)
				}
				return
			}
			if logger != nil {
				logger.Info("subscribed to NATS config notifications", "subject", subj)
			}
			select {
			case <-ctx.Done():
				_ = sub.Unsubscribe()
				return
			case <-sig:
				_ = sub.Unsubscribe()
			}
		}
	}()
}

// RunJWTRefreshLoop re-registers before NATS JWT expiry and reconnects.
//
//nolint:gocognit,gocyclo // explicit refresh steps
func (r *NatsRuntime) RunJWTRefreshLoop(ctx context.Context, logger *slog.Logger, cfg *Config, bootstrap *BootstrapData) {
	if r == nil || cfg == nil || bootstrap == nil {
		return
	}
	ticker := time.NewTicker(natsRefreshCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if bootstrap.Nats == nil || bootstrap.Nats.JWTExpiresAt == "" {
				continue
			}
			exp, err := time.Parse(time.RFC3339, bootstrap.Nats.JWTExpiresAt)
			if err != nil {
				continue
			}
			if time.Until(exp) > natsJWTRefreshLead {
				continue
			}
			newB, err := register(ctx, cfg)
			if err != nil {
				if logger != nil {
					logger.Warn("nats jwt refresh: re-register failed", "error", err)
				}
				continue
			}
			bootstrap.NodeJWT = newB.NodeJWT
			bootstrap.ExpiresAt = newB.ExpiresAt
			bootstrap.NodeReportURL = newB.NodeReportURL
			bootstrap.NodeConfigURL = newB.NodeConfigURL
			bootstrap.Nats = newB.Nats
			if newB.Nats == nil {
				if logger != nil {
					logger.Warn("nats jwt refresh: bootstrap missing nats block")
				}
				continue
			}
			ncfg := &natsutil.NatsConfig{ClientCredentials: *newB.Nats}
			if err := ncfg.Validate(); err != nil {
				if logger != nil {
					logger.Warn("nats jwt refresh: invalid nats config", "error", err)
				}
				continue
			}
			nc, js, err := natsutil.Connect(ncfg)
			if err != nil {
				if logger != nil {
					logger.Warn("nats jwt refresh: connect failed", "error", err)
				}
				continue
			}
			r.mu.Lock()
			old := r.nc
			r.nc = nc
			r.js = js
			r.cfg = ncfg
			r.mu.Unlock()
			if old != nil {
				_ = natsutil.CloseConn(old)
			}
			select {
			case r.reconnectSig <- struct{}{}:
			default:
			}
			if logger != nil {
				logger.Info("nats jwt refreshed and reconnected")
			}
		}
	}
}

// ParseNodeIDFromNodeJWT returns the node UUID from the HS256 node JWT `sub` claim without signature verification.
func ParseNodeIDFromNodeJWT(nodeJWT string) (uuid.UUID, error) {
	nodeJWT = strings.TrimSpace(nodeJWT)
	if nodeJWT == "" {
		return uuid.Nil, errors.New("empty node jwt")
	}
	parts := strings.Split(nodeJWT, ".")
	if len(parts) != 3 {
		return uuid.Nil, errors.New("invalid node jwt format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return uuid.Nil, fmt.Errorf("decode jwt payload: %w", err)
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return uuid.Nil, fmt.Errorf("parse jwt claims: %w", err)
	}
	return uuid.Parse(strings.TrimSpace(claims.Sub))
}
