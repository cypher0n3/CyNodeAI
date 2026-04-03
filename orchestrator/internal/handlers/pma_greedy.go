// Package handlers: greedy PMA provisioning on interactive session (REQ-ORCHES-0190).
package handlers

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// PMACredentialInvocationClassGatewaySession is the MCP invocation class for gateway-mediated PMA sessions.
// See CYNAI.MCPGAT.PmaInvocationClass (mcp_gateway_enforcement.md).
const PMACredentialInvocationClassGatewaySession = "user_gateway_session"

// GreedyPMAIssue records the last greedy provisioning outcome for tests and diagnostics.
type GreedyPMAIssue struct {
	BindingKey        string
	ServiceID         string
	InvocationClass   string
	ConfigVersionULID string
}

var greedyPMAIssue *GreedyPMAIssue

// ResetGreedyPMAIssueForTest clears the recorded greedy issue (handlers tests only).
func ResetGreedyPMAIssueForTest() {
	greedyPMAIssue = nil
}

// LastGreedyPMAIssueForTest returns the last greedy issue, or nil.
func LastGreedyPMAIssueForTest() *GreedyPMAIssue {
	return greedyPMAIssue
}

// GreedyProvisionPMAAfterInteractiveSession upserts the session binding, records MCP credential intent
// for the gateway session, and bumps config on the PMA host node so workers pull updated desired state.
func GreedyProvisionPMAAfterInteractiveSession(ctx context.Context, db database.Store, userID, interactiveSessionID uuid.UUID, logger *slog.Logger) error {
	lineage := models.SessionBindingLineage{UserID: userID, SessionID: interactiveSessionID, ThreadID: nil}
	key := models.DeriveSessionBindingKey(lineage)
	svcID, pickErr := pickPMAPoolServiceIDForGreedy(ctx, db, userID, interactiveSessionID, logger)
	if pickErr != nil {
		return pickErr
	}
	if _, err := db.UpsertSessionBinding(ctx, lineage, svcID, models.SessionBindingStateActive); err != nil {
		return err
	}
	greedyPMAIssue = &GreedyPMAIssue{
		BindingKey:      key,
		ServiceID:       svcID,
		InvocationClass: PMACredentialInvocationClassGatewaySession,
	}
	newVer, err := BumpPMAHostConfigVersion(ctx, db, logger)
	if err != nil {
		return err
	}
	greedyPMAIssue.ConfigVersionULID = newVer
	return nil
}
