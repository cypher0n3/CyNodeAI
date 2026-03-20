package testutil

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func TestMockDB_GetOrCreateActiveChatThread(t *testing.T) {
	m := NewMockDB()
	ctx := context.Background()
	userID := uuid.New()
	thread, err := m.GetOrCreateActiveChatThread(ctx, userID, nil)
	if err != nil {
		t.Fatalf("GetOrCreateActiveChatThread: %v", err)
	}
	if thread.ID == uuid.Nil || thread.UserID != userID || thread.ProjectID != nil {
		t.Errorf("thread: %+v", thread)
	}
	projID := uuid.New()
	thread2, err := m.GetOrCreateActiveChatThread(ctx, userID, &projID)
	if err != nil {
		t.Fatalf("GetOrCreateActiveChatThread with project: %v", err)
	}
	if thread2.ProjectID == nil || *thread2.ProjectID != projID {
		t.Errorf("thread2.ProjectID: %v", thread2.ProjectID)
	}
}

func TestMockDB_AppendChatMessage(t *testing.T) {
	m := NewMockDB()
	ctx := context.Background()
	userID := uuid.New()
	thread, _ := m.GetOrCreateActiveChatThread(ctx, userID, nil)
	msg, err := m.AppendChatMessage(ctx, thread.ID, "user", "hello", nil)
	if err != nil {
		t.Fatalf("AppendChatMessage: %v", err)
	}
	if msg.Role != "user" || msg.Content != "hello" {
		t.Errorf("message: %+v", msg)
	}
	meta := `{"model_id":"x"}`
	msg2, err := m.AppendChatMessage(ctx, thread.ID, "assistant", "hi", &meta)
	if err != nil {
		t.Fatalf("AppendChatMessage 2: %v", err)
	}
	if msg2.Content != "hi" || msg2.Metadata == nil || *msg2.Metadata != meta {
		t.Errorf("message2: %+v", msg2)
	}
}

func TestMockDB_CreateChatAuditLog(t *testing.T) {
	m := NewMockDB()
	ctx := context.Background()
	rec := &models.ChatAuditLog{
		ChatAuditLogBase: models.ChatAuditLogBase{
			Outcome:          "success",
			RedactionApplied: true,
		},
	}
	if err := m.CreateChatAuditLog(ctx, rec); err != nil {
		t.Fatalf("CreateChatAuditLog: %v", err)
	}
}
