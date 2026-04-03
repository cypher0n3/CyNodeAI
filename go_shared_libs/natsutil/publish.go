package natsutil

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

const (
	eventSessionActivity = "session.activity"
	eventSessionAttached = "session.attached"
	eventSessionDetached = "session.detached"
	eventNodeConfig      = "node.config_changed"
)

// PublishSessionActivity publishes session.activity to JetStream on the canonical subject and mirrors
// the same payload on core NATS when nc is non-nil (session user JWTs may subscribe without JetStream API).
func PublishSessionActivity(nc *nats.Conn, js nats.JetStreamContext, tenantID, sessionID string, producer Producer, scope Scope, correlation Correlation, payload *SessionActivityPayloadV1) error { //nolint:dupl // typed API wrapper
	subj := fmt.Sprintf("cynode.session.activity.%s.%s", tenantID, sessionID)
	return publishPayload(nc, js, subj, eventSessionActivity, EventVersionSessionV1, producer, scope, correlation, payload)
}

// PublishSessionAttached publishes session.attached.
func PublishSessionAttached(nc *nats.Conn, js nats.JetStreamContext, tenantID, sessionID string, producer Producer, scope Scope, correlation Correlation, payload *SessionAttachedPayloadV1) error { //nolint:dupl // typed API wrapper
	subj := fmt.Sprintf("cynode.session.attached.%s.%s", tenantID, sessionID)
	return publishPayload(nc, js, subj, eventSessionAttached, EventVersionSessionV1, producer, scope, correlation, payload)
}

// PublishSessionDetached publishes session.detached.
func PublishSessionDetached(nc *nats.Conn, js nats.JetStreamContext, tenantID, sessionID string, producer Producer, scope Scope, correlation Correlation, payload *SessionDetachedPayloadV1) error { //nolint:dupl // typed API wrapper
	subj := fmt.Sprintf("cynode.session.detached.%s.%s", tenantID, sessionID)
	return publishPayload(nc, js, subj, eventSessionDetached, EventVersionSessionV1, producer, scope, correlation, payload)
}

// PublishConfigChanged publishes node.config_changed.
func PublishConfigChanged(js nats.JetStreamContext, tenantID, nodeID string, producer Producer, scope Scope, correlation Correlation, payload *NodeConfigChangedPayloadV1) error { //nolint:dupl // typed API wrapper
	subj := fmt.Sprintf("cynode.node.config_changed.%s.%s", tenantID, nodeID)
	return publishPayload(nil, js, subj, eventNodeConfig, EventVersionConfigV1, producer, scope, correlation, payload)
}

func publishPayload(nc *nats.Conn, js nats.JetStreamContext, subject, eventType, eventVersion string, producer Producer, scope Scope, correlation Correlation, payload any) error {
	if payload == nil {
		return fmt.Errorf("natsutil: nil payload")
	}
	env, err := buildEnvelope(eventType, eventVersion, producer, scope, correlation, payload)
	if err != nil {
		return err
	}
	return publishJSON(js, nc, subject, &env)
}

func buildEnvelope(eventType, eventVersion string, producer Producer, scope Scope, correlation Correlation, payload any) (Envelope, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return Envelope{}, err
	}
	return Envelope{
		EventID:      uuid.New().String(),
		EventType:    eventType,
		EventVersion: eventVersion,
		OccurredAt:   time.Now().UTC().Format(time.RFC3339),
		Producer:     producer,
		Scope:        scope,
		Correlation:  correlation,
		Payload:      m,
	}, nil
}

func publishJSON(js nats.JetStreamContext, nc *nats.Conn, subject string, env *Envelope) error {
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	if _, err := js.Publish(subject, b); err != nil {
		return err
	}
	if nc != nil {
		if err := nc.Publish(subject, b); err != nil {
			return fmt.Errorf("natsutil: core mirror publish: %w", err)
		}
	}
	return nil
}
