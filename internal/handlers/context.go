package handlers

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const (
	contextKeyUserID   contextKey = "user_id"
	contextKeyHandle   contextKey = "handle"
	contextKeyNodeID   contextKey = "node_id"
	contextKeyNodeSlug contextKey = "node_slug"
)

// SetUserContext adds user info to context.
func SetUserContext(ctx context.Context, userID uuid.UUID, handle string) context.Context {
	ctx = context.WithValue(ctx, contextKeyUserID, userID)
	ctx = context.WithValue(ctx, contextKeyHandle, handle)
	return ctx
}

// SetNodeContext adds node info to context.
func SetNodeContext(ctx context.Context, nodeID uuid.UUID, nodeSlug string) context.Context {
	ctx = context.WithValue(ctx, contextKeyNodeID, nodeID)
	ctx = context.WithValue(ctx, contextKeyNodeSlug, nodeSlug)
	return ctx
}

func getUserIDFromContext(ctx context.Context) *uuid.UUID {
	if id, ok := ctx.Value(contextKeyUserID).(uuid.UUID); ok {
		return &id
	}
	return nil
}

func getNodeIDFromContext(ctx context.Context) *uuid.UUID {
	if id, ok := ctx.Value(contextKeyNodeID).(uuid.UUID); ok {
		return &id
	}
	return nil
}
