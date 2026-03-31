package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

const chatThreadInactivityThreshold = 2 * time.Hour

const (
	// DefaultChatMessagePageLimit is the default page size for ListChatMessages when limit <= 0.
	DefaultChatMessagePageLimit = 50
	// MaxChatMessagePageLimit is the maximum page size for ListChatMessages.
	MaxChatMessagePageLimit    = 100
	defaultChatThreadPageLimit = 20
	maxChatThreadPageLimit     = 100
)

// GetOrCreateActiveChatThread returns the most recent chat thread for (userID, projectID) if updated within 2h; otherwise creates a new thread.
// See docs/tech_specs/openai_compatible_chat_api.md and chat_threads_and_messages.md.
func (db *DB) GetOrCreateActiveChatThread(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID) (*models.ChatThread, error) {
	cutoff := time.Now().UTC().Add(-chatThreadInactivityThreshold)
	q := db.db.WithContext(ctx).Model(&ChatThreadRecord{}).Where("user_id = ?", userID).Order("updated_at DESC").Limit(1)
	if projectID == nil {
		q = q.Where("project_id IS NULL")
	} else {
		q = q.Where("project_id = ?", *projectID)
	}
	var record ChatThreadRecord
	err := q.First(&record).Error
	if err == nil && record.UpdatedAt.After(cutoff) {
		return record.ToChatThread(), nil
	}
	now := time.Now().UTC()
	newRecord := &ChatThreadRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
		},
		ChatThreadBase: models.ChatThreadBase{
			UserID:    userID,
			ProjectID: projectID,
		},
	}
	if err := db.createRecord(ctx, newRecord, "create chat thread"); err != nil {
		return nil, err
	}
	return newRecord.ToChatThread(), nil
}

// CreateChatThread unconditionally creates a new chat thread for (userID, projectID). Title is optional.
func (db *DB) CreateChatThread(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, title *string) (*models.ChatThread, error) {
	now := time.Now().UTC()
	record := &ChatThreadRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
		},
		ChatThreadBase: models.ChatThreadBase{
			UserID:    userID,
			ProjectID: projectID,
			Title:     title,
		},
	}
	if err := db.createRecord(ctx, record, "create chat thread"); err != nil {
		return nil, err
	}
	return record.ToChatThread(), nil
}

// AppendChatMessage adds a message to the thread and updates the thread's updated_at.
func (db *DB) AppendChatMessage(ctx context.Context, threadID uuid.UUID, role, content string, metadata *string) (*models.ChatMessage, error) {
	now := time.Now().UTC()
	record := &ChatMessageRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        uuid.New(),
			CreatedAt: now,
		},
		ChatMessageBase: models.ChatMessageBase{
			ThreadID: threadID,
			Role:     role,
			Content:  content,
			Metadata: metadata,
		},
	}
	if err := db.createRecord(ctx, record, "append chat message"); err != nil {
		return nil, err
	}
	if err := db.db.WithContext(ctx).Model(&ChatThreadRecord{}).Where("id = ?", threadID).Update("updated_at", now).Error; err != nil {
		return record.ToChatMessage(), wrapErr(err, "update chat thread updated_at")
	}
	return record.ToChatMessage(), nil
}

// ListChatMessages returns one page of messages for threadID ordered oldest-first and the total message count.
// limit<=0 uses DefaultChatMessagePageLimit; limit is clamped to MaxChatMessagePageLimit.
func (db *DB) ListChatMessages(ctx context.Context, threadID uuid.UUID, limit, offset int) ([]*models.ChatMessage, int64, error) {
	var total int64
	if err := db.db.WithContext(ctx).Model(&ChatMessageRecord{}).Where("thread_id = ?", threadID).Count(&total).Error; err != nil {
		return nil, 0, wrapErr(err, "count chat messages")
	}
	if limit <= 0 {
		limit = DefaultChatMessagePageLimit
	}
	if limit > MaxChatMessagePageLimit {
		limit = MaxChatMessagePageLimit
	}
	if offset < 0 {
		offset = 0
	}
	var records []ChatMessageRecord
	err := db.db.WithContext(ctx).Model(&ChatMessageRecord{}).
		Where("thread_id = ?", threadID).
		Order("created_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error
	if err != nil {
		return nil, 0, wrapErr(err, "list chat messages")
	}
	msgs := make([]*models.ChatMessage, len(records))
	for i := range records {
		msgs[i] = records[i].ToChatMessage()
	}
	return msgs, total, nil
}

// CreateChatAuditLog writes a chat completion audit record.
func (db *DB) CreateChatAuditLog(ctx context.Context, rec *models.ChatAuditLog) error {
	record := &ChatAuditLogRecord{
		ChatAuditLogBase: models.ChatAuditLogBase{
			UserID:           rec.UserID,
			ProjectID:        rec.ProjectID,
			Outcome:          rec.Outcome,
			ErrorCode:        rec.ErrorCode,
			RedactionApplied: rec.RedactionApplied,
			RedactionKinds:   rec.RedactionKinds,
			DurationMs:       rec.DurationMs,
			RequestID:        rec.RequestID,
		},
	}
	return insertAuditModel(db, ctx, record, record, rec, "create chat audit log")
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
	var record ChatMessageRecord
	err = db.db.WithContext(ctx).Model(&ChatMessageRecord{}).
		Where("metadata @> ?::jsonb", string(meta)).
		Limit(1).
		First(&record).Error
	if err != nil {
		return nil, wrapErr(err, "get thread by response id")
	}
	return db.GetChatThreadByID(ctx, record.ThreadID, userID)
}

// ListChatThreads returns threads for the user ordered by updated_at DESC. Optional projectID filter (when set, only that project).
// limit<=0 uses defaultChatThreadPageLimit; limit is clamped to maxChatThreadPageLimit.
func (db *DB) ListChatThreads(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, limit, offset int) ([]*models.ChatThread, error) {
	if limit <= 0 {
		limit = defaultChatThreadPageLimit
	}
	if limit > maxChatThreadPageLimit {
		limit = maxChatThreadPageLimit
	}
	if offset < 0 {
		offset = 0
	}
	q := db.db.WithContext(ctx).Model(&ChatThreadRecord{}).Where("user_id = ?", userID).Order("updated_at DESC")
	if projectID != nil {
		q = q.Where("project_id = ?", *projectID)
	}
	q = q.Limit(limit).Offset(offset)
	var records []ChatThreadRecord
	if err := q.Find(&records).Error; err != nil {
		return nil, wrapErr(err, "list chat threads")
	}
	threads := make([]*models.ChatThread, len(records))
	for i := range records {
		threads[i] = records[i].ToChatThread()
	}
	return threads, nil
}

// GetChatThreadByID returns the thread if it belongs to the user.
func (db *DB) GetChatThreadByID(ctx context.Context, threadID, userID uuid.UUID) (*models.ChatThread, error) {
	var record ChatThreadRecord
	err := db.db.WithContext(ctx).Where("id = ? AND user_id = ?", threadID, userID).First(&record).Error
	if err != nil {
		return nil, wrapErr(err, "get chat thread")
	}
	return record.ToChatThread(), nil
}

// UpdateChatThreadTitle updates the thread title. Returns ErrNotFound if thread does not belong to user.
func (db *DB) UpdateChatThreadTitle(ctx context.Context, threadID, userID uuid.UUID, title string) error {
	res := db.db.WithContext(ctx).Model(&ChatThreadRecord{}).
		Where("id = ? AND user_id = ?", threadID, userID).
		Update("title", title)
	if res.Error != nil {
		return wrapErr(res.Error, "update chat thread title")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	now := time.Now().UTC()
	_ = db.db.WithContext(ctx).Model(&ChatThreadRecord{}).Where("id = ?", threadID).Update("updated_at", now)
	return nil
}
