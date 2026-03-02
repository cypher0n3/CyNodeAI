package handlers

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type contextSetter func(context.Context, uuid.UUID, string) context.Context
type contextGetter func(context.Context) *uuid.UUID

func assertContextSetGet(t *testing.T, set contextSetter, get contextGetter, id uuid.UUID, label string) {
	t.Helper()
	ctx := set(context.Background(), id, label)
	got := get(ctx)
	if got == nil {
		t.Fatal("get from context returned nil")
		return
	}
	if *got != id {
		t.Errorf("get from context = %v, want %v", *got, id)
	}
}

func assertContextEmpty(t *testing.T, get contextGetter) {
	t.Helper()
	if get(context.Background()) != nil {
		t.Error("get on empty context should return nil")
	}
}

func TestSetUserContext(t *testing.T) {
	assertContextSetGet(t, SetUserContext, getUserIDFromContext, uuid.New(), "testuser")
}

func TestSetNodeContext(t *testing.T) {
	assertContextSetGet(t, SetNodeContext, getNodeIDFromContext, uuid.New(), "test-node")
}

func TestGetUserIDFromContext_Empty(t *testing.T) {
	assertContextEmpty(t, getUserIDFromContext)
}

func TestGetNodeIDFromContext_Empty(t *testing.T) {
	assertContextEmpty(t, getNodeIDFromContext)
}

func TestGetHandleFromContext(t *testing.T) {
	ctx := SetUserContext(context.Background(), uuid.New(), "admin")
	if got := GetHandleFromContext(ctx); got != "admin" {
		t.Errorf("GetHandleFromContext = %q, want admin", got)
	}
	if got := GetHandleFromContext(context.Background()); got != "" {
		t.Errorf("GetHandleFromContext(empty) = %q, want empty", got)
	}
}
