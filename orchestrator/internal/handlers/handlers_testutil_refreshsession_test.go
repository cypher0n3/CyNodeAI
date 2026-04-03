package handlers

import (
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
	"github.com/google/uuid"
)

// addTestRefreshSession inserts an active, non-expired refresh session (handler tests).
func addTestRefreshSession(t *testing.T, db *testutil.MockDB, userID, sessionID uuid.UUID, tokenHash []byte) {
	t.Helper()
	db.AddRefreshSession(&models.RefreshSession{
		RefreshSessionBase: models.RefreshSessionBase{
			UserID:           userID,
			RefreshTokenHash: tokenHash,
			IsActive:         true,
			ExpiresAt:        time.Now().UTC().Add(time.Hour),
		},
		ID:        sessionID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
}

// putMockSessionBindingByLineage adds an active binding keyed by lineage plus a matching refresh session.
func putMockSessionBindingByLineage(t *testing.T, db *testutil.MockDB, userID, sessionID uuid.UUID, serviceID string, tokenHash []byte) {
	t.Helper()
	lineage := models.SessionBindingLineage{UserID: userID, SessionID: sessionID, ThreadID: nil}
	key := models.DeriveSessionBindingKey(lineage)
	db.SessionBindingsByKey[key] = &models.SessionBinding{
		SessionBindingBase: models.SessionBindingBase{
			BindingKey: key,
			UserID:     userID,
			SessionID:  sessionID,
			ServiceID:  serviceID,
			State:      models.SessionBindingStateActive,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	addTestRefreshSession(t, db, userID, sessionID, tokenHash)
}
