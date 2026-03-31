// Package handlers: PMA teardown for stale bindings (REQ-ORCHES-0191).
package handlers

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// PMATeardownRecord captures the last teardown for tests and diagnostics.
type PMATeardownRecord struct {
	BindingKey              string
	ServiceID               string
	Reason                  string
	MCPInvalidationRecorded bool
	ConfigVersionULID       string
}

var lastPMATeardown *PMATeardownRecord

// ResetPMATeardownForTest clears teardown test state.
func ResetPMATeardownForTest() {
	lastPMATeardown = nil
}

// LastPMATeardownForTest returns the last teardown record, or nil.
func LastPMATeardownForTest() *PMATeardownRecord {
	return lastPMATeardown
}

// TeardownPMAForInteractiveSession marks the binding teardown-pending, records MCP invalidation intent, and bumps node config.
func TeardownPMAForInteractiveSession(ctx context.Context, db database.Store, userID, interactiveSessionID uuid.UUID, reason string, logger *slog.Logger) error {
	lineage := models.SessionBindingLineage{UserID: userID, SessionID: interactiveSessionID, ThreadID: nil}
	key := models.DeriveSessionBindingKey(lineage)
	existing, err := db.GetSessionBindingByKey(ctx, key)
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if existing.State != models.SessionBindingStateActive {
		return nil
	}
	if _, err := db.UpsertSessionBinding(ctx, lineage, existing.ServiceID, models.SessionBindingStateTeardownPending); err != nil {
		return err
	}
	newVer, berr := BumpPMAHostConfigVersion(ctx, db, logger)
	if berr != nil {
		return berr
	}
	lastPMATeardown = &PMATeardownRecord{
		BindingKey:              key,
		ServiceID:               existing.ServiceID,
		Reason:                  reason,
		MCPInvalidationRecorded: true,
		ConfigVersionULID:       newVer,
	}
	return nil
}

// TeardownAllActivePMABindingsForUser tears down every active PMA binding for the user (e.g. admin revoke).
func TeardownAllActivePMABindingsForUser(ctx context.Context, db database.Store, userID uuid.UUID, reason string, logger *slog.Logger) error {
	bindings, err := db.ListActiveBindingsForUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, b := range bindings {
		if err := TeardownPMAForInteractiveSession(ctx, db, userID, b.SessionID, reason, logger); err != nil {
			return err
		}
	}
	return nil
}

// TouchPMABindingActivity updates last activity for active bindings (PMA chat / gateway use).
func TouchPMABindingActivity(ctx context.Context, db database.SessionBindingStore, userID uuid.UUID) error {
	return db.TouchActiveSessionBindingsForUser(ctx, userID, time.Now().UTC())
}
