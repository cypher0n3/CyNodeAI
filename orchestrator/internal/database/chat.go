package database

import (
	"context"
	"encoding/json"
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

// CreateChatThread unconditionally creates a new chat thread for (userID, projectID). Title is optional.
func (db *DB) CreateChatThread(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, title *string) (*models.ChatThread, error) {
	now := time.Now().UTC()
	t := &models.ChatThread{
		ID:        uuid.New(),
		UserID:    userID,
		ProjectID: projectID,
		Title:     title,
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

// GetThreadByResponseID resolves previous_response_id to the thread that owns that response.
// The response_id is stored in assistant message metadata when using POST /v1/responses.
// Uses jsonb containment (one param only) then GetChatThreadByID so pgx never sees uuid-vs-text in one query.
func (db *DB) GetThreadByResponseID(ctx context.Context, responseID string, userID uuid.UUID) (*models.ChatThread, error) {
	if responseID == "" {
		return nil, ErrNotFound
	}
	meta, err := json.Marshal(map[string]string{"response_id": responseID})
	if err != nil {
		return nil, wrapErr(err, "get thread by response id")
	}
	var msg models.ChatMessage
	err = db.db.WithContext(ctx).Model(&models.ChatMessage{}).
		Where("metadata @> ?::jsonb", string(meta)).
		Limit(1).
		First(&msg).Error
	if err != nil {
		return nil, wrapErr(err, "get thread by response id")
	}
	return db.GetChatThreadByID(ctx, msg.ThreadID, userID)
}

// ListChatThreads returns threads for the user ordered by updated_at DESC. Optional projectID filter (when set, only that project).
func (db *DB) ListChatThreads(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, limit, offset int) ([]*models.ChatThread, error) {
	q := db.db.WithContext(ctx).Where("user_id = ?", userID).Order("updated_at DESC")
	if projectID != nil {
		q = q.Where("project_id = ?", *projectID)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	var threads []*models.ChatThread
	if err := q.Find(&threads).Error; err != nil {
		return nil, wrapErr(err, "list chat threads")
	}
	return threads, nil
}

// GetChatThreadByID returns the thread if it belongs to the user.
func (db *DB) GetChatThreadByID(ctx context.Context, threadID, userID uuid.UUID) (*models.ChatThread, error) {
	var t models.ChatThread
	err := db.db.WithContext(ctx).Where("id = ? AND user_id = ?", threadID, userID).First(&t).Error
	if err != nil {
		return nil, wrapErr(err, "get chat thread")
	}
	return &t, nil
}

// UpdateChatThreadTitle updates the thread title. Returns ErrNotFound if thread does not belong to user.
func (db *DB) UpdateChatThreadTitle(ctx context.Context, threadID, userID uuid.UUID, title string) error {
	res := db.db.WithContext(ctx).Model(&models.ChatThread{}).
		Where("id = ? AND user_id = ?", threadID, userID).
		Update("title", title)
	if res.Error != nil {
		return wrapErr(res.Error, "update chat thread title")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	now := time.Now().UTC()
	_ = db.db.WithContext(ctx).Model(&models.ChatThread{}).Where("id = ?", threadID).Update("updated_at", now)
	return nil
}
