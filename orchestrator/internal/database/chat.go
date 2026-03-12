package database

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

const chatThreadInactivityThreshold = 2 * time.Hour

// GetOrCreateActiveChatThread returns the most recent chat thread for (userID, projectID) if updated within 2h; otherwise creates a new thread.
// See docs/tech_specs/openai_compatible_chat_api.md and chat_threads_and_messages.md.
func (db *DB) GetOrCreateActiveChatThread(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID) (*models.ChatThread, error) {
	cutoff := time.Now().UTC().Add(-chatThreadInactivityThreshold)
	q := db.db.WithContext(ctx).Where("user_id = ?", userID).Order("updated_at DESC").Limit(1)
	if projectID == nil {
		q = q.Where("project_id IS NULL")
	} else {
		q = q.Where("project_id = ?", *projectID)
	}
	var thread models.ChatThread
	err := q.First(&thread).Error
	if err == nil && thread.UpdatedAt.After(cutoff) {
		return &thread, nil
	}
	now := time.Now().UTC()
	newThread := &models.ChatThread{
		ID:        uuid.New(),
		UserID:    userID,
		ProjectID: projectID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.createRecord(ctx, newThread, "create chat thread"); err != nil {
		return nil, err
	}
	return newThread, nil
}

// CreateChatThread unconditionally creates a new chat thread for (userID, projectID).
func (db *DB) CreateChatThread(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID) (*models.ChatThread, error) {
	now := time.Now().UTC()
	t := &models.ChatThread{
		ID:        uuid.New(),
		UserID:    userID,
		ProjectID: projectID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.createRecord(ctx, t, "create chat thread"); err != nil {
		return nil, err
	}
	return t, nil
}

// AppendChatMessage adds a message to the thread and updates the thread's updated_at.
func (db *DB) AppendChatMessage(ctx context.Context, threadID uuid.UUID, role, content string, metadata *string) (*models.ChatMessage, error) {
	now := time.Now().UTC()
	msg := &models.ChatMessage{
		ID:        uuid.New(),
		ThreadID:  threadID,
		Role:      role,
		Content:   content,
		Metadata:  metadata,
		CreatedAt: now,
	}
	if err := db.createRecord(ctx, msg, "append chat message"); err != nil {
		return nil, err
	}
	if err := db.db.WithContext(ctx).Model(&models.ChatThread{}).Where("id = ?", threadID).Update("updated_at", now).Error; err != nil {
		return msg, wrapErr(err, "update chat thread updated_at")
	}
	return msg, nil
}

// ListChatMessages returns up to limit messages for threadID ordered oldest-first.
// A limit of 0 returns all messages.
func (db *DB) ListChatMessages(ctx context.Context, threadID uuid.UUID, limit int) ([]*models.ChatMessage, error) {
	q := db.db.WithContext(ctx).Where("thread_id = ?", threadID).Order("created_at ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var msgs []*models.ChatMessage
	if err := q.Find(&msgs).Error; err != nil {
		return nil, wrapErr(err, "list chat messages")
	}
	return msgs, nil
}

// CreateChatAuditLog writes a chat completion audit record.
func (db *DB) CreateChatAuditLog(ctx context.Context, rec *models.ChatAuditLog) error {
	ensureAuditIDAndTime(&rec.ID, &rec.CreatedAt)
	return db.createRecord(ctx, rec, "create chat audit log")
}
