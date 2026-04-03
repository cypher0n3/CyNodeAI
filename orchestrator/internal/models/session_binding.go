// Package models: session binding for per-binding PMA provisioning (REQ-ORCHES-0188).
package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Session binding state for a PMA instance tied to an interactive session binding.
const (
	SessionBindingStateActive          = "active"
	SessionBindingStateTeardownPending = "teardown_pending"
)

// SessionBindingLineage identifies the interactive session for PMA binding (user + session + optional chat thread).
// See CYNAI.ORCHES.PmaInstancePerSessionBinding (orchestrator_bootstrap.md).
type SessionBindingLineage struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	ThreadID  *uuid.UUID
}

// SessionBindingBase is the domain base struct for SessionBinding (without ID/timestamps).
type SessionBindingBase struct {
	BindingKey     string     `gorm:"column:binding_key;uniqueIndex;not null" json:"binding_key"`
	UserID         uuid.UUID  `gorm:"column:user_id;index" json:"user_id"`
	SessionID      uuid.UUID  `gorm:"column:session_id;index" json:"session_id"`
	ThreadID       *uuid.UUID `gorm:"column:thread_id" json:"thread_id,omitempty"`
	ServiceID      string     `gorm:"column:service_id" json:"service_id"`
	State          string     `gorm:"column:state;index" json:"state"`
	LastActivityAt *time.Time `gorm:"column:last_activity_at" json:"last_activity_at,omitempty"`
}

// SessionBinding is a persisted PMA session binding row (orchestrator control plane).
type SessionBinding struct {
	SessionBindingBase
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DeriveSessionBindingKey returns a stable opaque key from user ID and session/thread lineage.
// The same lineage always yields the same key; different users, sessions, or threads yield different keys.
//
// PMAServiceIDForBindingKey is legacy: production assigns pma-pool-* slots (REQ-ORCHES-0192), not
// per-binding pma-sb-* ids. Retained for tests and migration helpers.
func PMAServiceIDForBindingKey(bindingKey string) string {
	if len(bindingKey) >= 12 {
		return "pma-sb-" + bindingKey[:12]
	}
	return "pma-sb-" + bindingKey
}

func DeriveSessionBindingKey(lineage SessionBindingLineage) string {
	threadPart := uuid.Nil.String()
	if lineage.ThreadID != nil {
		threadPart = lineage.ThreadID.String()
	}
	// Canonical, versioned string for deterministic hashing.
	payload := fmt.Sprintf(
		"cynodeai.session_binding.v1\n%s\n%s\n%s",
		lineage.UserID.String(),
		lineage.SessionID.String(),
		threadPart,
	)
	sum := sha256.Sum256([]byte(payload))
	return strings.ToLower(hex.EncodeToString(sum[:]))
}
