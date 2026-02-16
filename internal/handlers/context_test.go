package handlers

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestSetUserContext(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	handle := "testuser"

	ctx = SetUserContext(ctx, userID, handle)

	retrievedID := getUserIDFromContext(ctx)
	if retrievedID == nil {
		t.Fatal("getUserIDFromContext() returned nil")
	}

	if *retrievedID != userID {
		t.Errorf("getUserIDFromContext() = %v, want %v", *retrievedID, userID)
	}
}

func TestSetNodeContext(t *testing.T) {
	ctx := context.Background()
	nodeID := uuid.New()
	nodeSlug := "test-node"

	ctx = SetNodeContext(ctx, nodeID, nodeSlug)

	retrievedID := getNodeIDFromContext(ctx)
	if retrievedID == nil {
		t.Fatal("getNodeIDFromContext() returned nil")
	}

	if *retrievedID != nodeID {
		t.Errorf("getNodeIDFromContext() = %v, want %v", *retrievedID, nodeID)
	}
}

func TestGetUserIDFromContext_Empty(t *testing.T) {
	ctx := context.Background()

	retrievedID := getUserIDFromContext(ctx)
	if retrievedID != nil {
		t.Error("getUserIDFromContext() on empty context should return nil")
	}
}

func TestGetNodeIDFromContext_Empty(t *testing.T) {
	ctx := context.Background()

	retrievedID := getNodeIDFromContext(ctx)
	if retrievedID != nil {
		t.Error("getNodeIDFromContext() on empty context should return nil")
	}
}
