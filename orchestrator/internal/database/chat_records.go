// Package database provides GORM record structs for chat-related tables.
package database

import (
	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// SessionRecord is the GORM record struct for the sessions table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain SessionBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type SessionRecord struct {
	gormmodel.GormModelUUID
	models.SessionBase
}

// TableName implements the GORM TableName interface.
func (SessionRecord) TableName() string {
	return "sessions"
}

// ToSession converts a SessionRecord to a domain Session with all fields populated.
func (r *SessionRecord) ToSession() *models.Session {
	return &models.Session{
		SessionBase: r.SessionBase,
		ID:          r.ID,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// ChatThreadRecord is the GORM record struct for the chat_threads table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain ChatThreadBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type ChatThreadRecord struct {
	gormmodel.GormModelUUID
	models.ChatThreadBase
}

// TableName implements the GORM TableName interface.
func (ChatThreadRecord) TableName() string {
	return "chat_threads"
}

// ToChatThread converts a ChatThreadRecord to a domain ChatThread with all fields populated.
func (r *ChatThreadRecord) ToChatThread() *models.ChatThread {
	return &models.ChatThread{
		ChatThreadBase: r.ChatThreadBase,
		ID:             r.ID,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

// ChatMessageRecord is the GORM record struct for the chat_messages table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain ChatMessageBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type ChatMessageRecord struct {
	gormmodel.GormModelUUID
	models.ChatMessageBase
}

// TableName implements the GORM TableName interface.
func (ChatMessageRecord) TableName() string {
	return "chat_messages"
}

// ToChatMessage converts a ChatMessageRecord to a domain ChatMessage with all fields populated.
func (r *ChatMessageRecord) ToChatMessage() *models.ChatMessage {
	return &models.ChatMessage{
		ChatMessageBase: r.ChatMessageBase,
		ID:              r.ID,
		CreatedAt:       r.CreatedAt,
	}
}
