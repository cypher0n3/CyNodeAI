// Package database: access control and API egress credential checks per REQ-APIEGR-0110--0113 and access_control.md.
package database

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// ActionApiCall and ResourceTypeProviderOperation are used by API Egress for policy and audit.
const ActionApiCall = "api.call"
const ResourceTypeProviderOperation = "api.provider_operation"

// ListAccessControlRulesForApiCall returns rules for the given subject and action/resource type, ordered by priority desc.
// Used by API Egress to evaluate provider+operation allow/deny. Caller matches resource_pattern to provider/operation.
func (db *DB) ListAccessControlRulesForApiCall(ctx context.Context, subjectType string, subjectID *uuid.UUID, action, resourceType string) ([]*models.AccessControlRule, error) {
	q := db.db.WithContext(ctx).Model(&AccessControlRuleRecord{}).Where("action = ? AND resource_type = ?", action, resourceType).
		Where("subject_type = ?", subjectType)
	if subjectID == nil {
		q = q.Where("subject_id IS NULL")
	} else {
		q = q.Where("subject_id = ?", *subjectID)
	}
	var records []AccessControlRuleRecord
	if err := q.Order("priority DESC").Find(&records).Error; err != nil {
		return nil, wrapErr(err, "list access control rules")
	}
	rules := make([]*models.AccessControlRule, len(records))
	for i := range records {
		rules[i] = records[i].ToAccessControlRule()
	}
	return rules, nil
}

// CreateAccessControlAuditLog writes one audit record. REQ-APIEGR-0119.
func (db *DB) CreateAccessControlAuditLog(ctx context.Context, rec *models.AccessControlAuditLog) error {
	row := accessControlAuditRecordFrom(rec)
	return insertAuditModel(db, ctx, row, row, rec, opCreateAccessControlAuditLog)
}

// HasActiveApiCredentialForUserAndProvider returns true if the user has at least one active credential for the provider.
// REQ-APIEGR-0113: credential must be authorized and active (expires_at not past).
func (db *DB) HasActiveApiCredentialForUserAndProvider(ctx context.Context, userID uuid.UUID, provider string) (bool, error) {
	var n int64
	now := time.Now().UTC()
	err := db.db.WithContext(ctx).Model(&ApiCredentialRecord{}).
		Where("owner_type = ? AND owner_id = ? AND provider = ? AND is_active = ?", "user", userID, provider, true).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Count(&n).Error
	if err != nil {
		return false, wrapErr(err, "has active api credential")
	}
	return n > 0, nil
}

// HasAnyActiveApiCredential returns true if at least one active (non-expired) API credential exists.
// Used by control-plane for inference-path readiness: external provider keys count as an inference path (REQ-ORCHES-0150, orchestrator_bootstrap.md).
func (db *DB) HasAnyActiveApiCredential(ctx context.Context) (bool, error) {
	var n int64
	now := time.Now().UTC()
	err := db.db.WithContext(ctx).Model(&ApiCredentialRecord{}).
		Where("is_active = ?", true).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Count(&n).Error
	if err != nil {
		return false, wrapErr(err, "has any active api credential")
	}
	return n > 0, nil
}
