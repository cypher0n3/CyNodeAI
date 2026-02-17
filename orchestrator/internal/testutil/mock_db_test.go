package testutil

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func TestNewMockDB(t *testing.T) {
	db := NewMockDB()
	if db == nil {
		t.Fatal("NewMockDB returned nil")
		return
	}
	if db.Users == nil {
		t.Error("Users map should be initialized")
	}
}

func TestMockDB_GetNotFound(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		run  func(db *MockDB) error
	}{
		{"GetUserByHandle", func(db *MockDB) error { _, err := db.GetUserByHandle(ctx, "nonexistent"); return err }},
		{"GetUserByID", func(db *MockDB) error { _, err := db.GetUserByID(ctx, uuid.New()); return err }},
		{"GetPasswordCredentialByUserID", func(db *MockDB) error { _, err := db.GetPasswordCredentialByUserID(ctx, uuid.New()); return err }},
		{"GetTaskByID", func(db *MockDB) error { _, err := db.GetTaskByID(ctx, uuid.New()); return err }},
		{"GetNodeBySlug", func(db *MockDB) error { _, err := db.GetNodeBySlug(ctx, "nonexistent"); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := NewMockDB()
			err := tc.run(db)
			if !errors.Is(err, database.ErrNotFound) {
				t.Errorf("expected ErrNotFound, got %v", err)
			}
		})
	}
}

func TestMockDB_CreateUser(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	email := "test@example.com"
	user, err := db.CreateUser(ctx, "testuser", &email)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if user.Handle != "testuser" {
		t.Errorf("expected handle 'testuser', got %s", user.Handle)
	}
	if *user.Email != email {
		t.Errorf("expected email %s, got %s", email, *user.Email)
	}
}

func TestMockDB_CreateUserWithForceError(t *testing.T) {
	db := NewMockDB()
	db.ForceError = errors.New("forced error")
	ctx := context.Background()

	_, err := db.CreateUser(ctx, "testuser", nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestMockDB_GetUserByHandle(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	// Create a user first
	_, _ = db.CreateUser(ctx, "testuser", nil)

	// Get the user
	user, err := db.GetUserByHandle(ctx, "testuser")
	if err != nil {
		t.Fatalf("GetUserByHandle failed: %v", err)
	}
	if user.Handle != "testuser" {
		t.Errorf("expected handle 'testuser', got %s", user.Handle)
	}
}

func TestMockDB_GetUserByID(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	user, _ := db.CreateUser(ctx, "testuser", nil)
	retrieved, err := db.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if retrieved.ID != user.ID {
		t.Errorf("ID mismatch")
	}
}

func TestMockDB_PasswordCredential(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	userID := uuid.New()
	hash := []byte("hashed-password")

	cred, err := db.CreatePasswordCredential(ctx, userID, hash, "argon2id")
	if err != nil {
		t.Fatalf("CreatePasswordCredential failed: %v", err)
	}
	if cred.UserID != userID {
		t.Error("UserID mismatch")
	}

	retrieved, err := db.GetPasswordCredentialByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetPasswordCredentialByUserID failed: %v", err)
	}
	if retrieved.HashAlg != "argon2id" {
		t.Errorf("expected hash alg 'argon2id', got %s", retrieved.HashAlg)
	}
}

func TestMockDB_RefreshSession(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	userID := uuid.New()
	tokenHash := []byte("token-hash")
	expiresAt := time.Now().Add(time.Hour)

	session, err := db.CreateRefreshSession(ctx, userID, tokenHash, expiresAt)
	if err != nil {
		t.Fatalf("CreateRefreshSession failed: %v", err)
	}
	if !session.IsActive {
		t.Error("session should be active")
	}

	retrieved, err := db.GetActiveRefreshSession(ctx, tokenHash)
	if err != nil {
		t.Fatalf("GetActiveRefreshSession failed: %v", err)
	}
	if retrieved.ID != session.ID {
		t.Error("session ID mismatch")
	}

	// Invalidate session
	err = db.InvalidateRefreshSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("InvalidateRefreshSession failed: %v", err)
	}

	// Should not find inactive session
	_, err = db.GetActiveRefreshSession(ctx, tokenHash)
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound for inactive session, got %v", err)
	}
}

func TestMockDB_GetActiveRefreshSessionExpired(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	userID := uuid.New()
	tokenHash := []byte("expired-token")
	expiresAt := time.Now().Add(-time.Hour) // Expired

	_, _ = db.CreateRefreshSession(ctx, userID, tokenHash, expiresAt)

	_, err := db.GetActiveRefreshSession(ctx, tokenHash)
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound for expired session, got %v", err)
	}
}

func TestMockDB_CreateAuthAuditLog(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	userID := uuid.New()
	ip := "127.0.0.1"
	ua := "Mozilla/5.0"
	details := "test details"

	err := db.CreateAuthAuditLog(ctx, &userID, "login", true, &ip, &ua, &details)
	if err != nil {
		t.Fatalf("CreateAuthAuditLog failed: %v", err)
	}
	if len(db.AuditLogs) != 1 {
		t.Errorf("expected 1 audit log, got %d", len(db.AuditLogs))
	}
}

func TestMockDB_Task(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	userID := uuid.New()
	task, err := db.CreateTask(ctx, &userID, "test prompt")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if task.Status != models.TaskStatusPending {
		t.Errorf("expected status pending, got %s", task.Status)
	}

	retrieved, err := db.GetTaskByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}
	if *retrieved.Prompt != "test prompt" {
		t.Errorf("prompt mismatch")
	}
}

func TestMockDB_GetJobsByTaskID(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	taskID := uuid.New()

	// No jobs initially
	jobs, err := db.GetJobsByTaskID(ctx, taskID)
	if err != nil {
		t.Fatalf("GetJobsByTaskID failed: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestMockDB_Node(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	node, err := db.CreateNode(ctx, "test-node")
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	if node.NodeSlug != "test-node" {
		t.Errorf("expected slug 'test-node', got %s", node.NodeSlug)
	}

	retrieved, err := db.GetNodeBySlug(ctx, "test-node")
	if err != nil {
		t.Fatalf("GetNodeBySlug failed: %v", err)
	}
	if retrieved.ID != node.ID {
		t.Error("node ID mismatch")
	}
}

func TestMockDB_UpdateNodeStatus(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	node, _ := db.CreateNode(ctx, "test-node")
	err := db.UpdateNodeStatus(ctx, node.ID, "active")
	if err != nil {
		t.Fatalf("UpdateNodeStatus failed: %v", err)
	}
	if node.Status != "active" {
		t.Errorf("expected status 'active', got %s", node.Status)
	}
}

func TestMockDB_UpdateNodeLastSeen(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	node, _ := db.CreateNode(ctx, "test-node")
	err := db.UpdateNodeLastSeen(ctx, node.ID)
	if err != nil {
		t.Fatalf("UpdateNodeLastSeen failed: %v", err)
	}
	if node.LastSeenAt == nil {
		t.Error("expected LastSeenAt to be set")
	}
}

func TestMockDB_SaveNodeCapabilitySnapshot(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	nodeID := uuid.New()
	err := db.SaveNodeCapabilitySnapshot(ctx, nodeID, `{"version":1}`)
	if err != nil {
		t.Fatalf("SaveNodeCapabilitySnapshot failed: %v", err)
	}
	if len(db.CapabilityHistory) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(db.CapabilityHistory))
	}
}

func TestMockDB_UpdateNodeCapability(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()

	node, _ := db.CreateNode(ctx, "test-node")
	err := db.UpdateNodeCapability(ctx, node.ID, "sha256:abc123")
	if err != nil {
		t.Fatalf("UpdateNodeCapability failed: %v", err)
	}
	if node.CapabilityHash == nil || *node.CapabilityHash != "sha256:abc123" {
		t.Error("capability hash not updated")
	}
}

func TestMockDB_AddHelpers(t *testing.T) {
	db := NewMockDB()

	// AddUser
	user := &models.User{
		ID:     uuid.New(),
		Handle: "added-user",
	}
	db.AddUser(user)
	if db.Users[user.ID] == nil {
		t.Error("AddUser failed")
	}

	// AddPasswordCredential
	cred := &models.PasswordCredential{
		ID:     uuid.New(),
		UserID: user.ID,
	}
	db.AddPasswordCredential(cred)
	if db.PasswordCreds[cred.UserID] == nil {
		t.Error("AddPasswordCredential failed")
	}

	// AddTask
	task := &models.Task{
		ID: uuid.New(),
	}
	db.AddTask(task)
	if db.Tasks[task.ID] == nil {
		t.Error("AddTask failed")
	}

	// AddJob
	job := &models.Job{
		ID:     uuid.New(),
		TaskID: task.ID,
	}
	db.AddJob(job)
	if db.Jobs[job.ID] == nil {
		t.Error("AddJob failed")
	}
	if len(db.JobsByTask[task.ID]) != 1 {
		t.Error("AddJob didn't update JobsByTask")
	}

	// AddNode
	node := &models.Node{
		ID:       uuid.New(),
		NodeSlug: "added-node",
	}
	db.AddNode(node)
	if db.Nodes[node.ID] == nil {
		t.Error("AddNode failed")
	}

	// AddRefreshSession
	session := &models.RefreshSession{
		ID:               uuid.New(),
		RefreshTokenHash: []byte("hash"),
	}
	db.AddRefreshSession(session)
	if db.RefreshSessions[session.ID] == nil {
		t.Error("AddRefreshSession failed")
	}
}

func TestMockDB_ForceError_UserOperations(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()
	db.ForceError = errors.New("forced error")

	_, err := db.CreateUser(ctx, "test", nil)
	if err == nil {
		t.Error("CreateUser should return forced error")
	}

	_, err = db.GetUserByHandle(ctx, "test")
	if err == nil {
		t.Error("GetUserByHandle should return forced error")
	}

	_, err = db.GetUserByID(ctx, uuid.New())
	if err == nil {
		t.Error("GetUserByID should return forced error")
	}
}

func TestMockDB_ForceError_CredentialOperations(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()
	db.ForceError = errors.New("forced error")

	_, err := db.CreatePasswordCredential(ctx, uuid.New(), nil, "")
	if err == nil {
		t.Error("CreatePasswordCredential should return forced error")
	}

	_, err = db.GetPasswordCredentialByUserID(ctx, uuid.New())
	if err == nil {
		t.Error("GetPasswordCredentialByUserID should return forced error")
	}
}

func TestMockDB_ForceError_SessionOperations(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()
	db.ForceError = errors.New("forced error")

	_, err := db.CreateRefreshSession(ctx, uuid.New(), nil, time.Now())
	if err == nil {
		t.Error("CreateRefreshSession should return forced error")
	}

	_, err = db.GetActiveRefreshSession(ctx, nil)
	if err == nil {
		t.Error("GetActiveRefreshSession should return forced error")
	}

	err = db.InvalidateRefreshSession(ctx, uuid.New())
	if err == nil {
		t.Error("InvalidateRefreshSession should return forced error")
	}

	err = db.CreateAuthAuditLog(ctx, nil, "", false, nil, nil, nil)
	if err == nil {
		t.Error("CreateAuthAuditLog should return forced error")
	}
}

func TestMockDB_ForceError_TaskOperations(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()
	db.ForceError = errors.New("forced error")

	_, err := db.CreateTask(ctx, nil, "")
	if err == nil {
		t.Error("CreateTask should return forced error")
	}

	_, err = db.GetTaskByID(ctx, uuid.New())
	if err == nil {
		t.Error("GetTaskByID should return forced error")
	}

	_, err = db.GetJobsByTaskID(ctx, uuid.New())
	if err == nil {
		t.Error("GetJobsByTaskID should return forced error")
	}
}

func TestMockDB_ForceError_NodeOperations(t *testing.T) {
	db := NewMockDB()
	ctx := context.Background()
	db.ForceError = errors.New("forced error")

	_, err := db.CreateNode(ctx, "")
	if err == nil {
		t.Error("CreateNode should return forced error")
	}

	_, err = db.GetNodeBySlug(ctx, "")
	if err == nil {
		t.Error("GetNodeBySlug should return forced error")
	}

	err = db.UpdateNodeStatus(ctx, uuid.New(), "")
	if err == nil {
		t.Error("UpdateNodeStatus should return forced error")
	}

	err = db.UpdateNodeLastSeen(ctx, uuid.New())
	if err == nil {
		t.Error("UpdateNodeLastSeen should return forced error")
	}

	err = db.SaveNodeCapabilitySnapshot(ctx, uuid.New(), "")
	if err == nil {
		t.Error("SaveNodeCapabilitySnapshot should return forced error")
	}

	err = db.UpdateNodeCapability(ctx, uuid.New(), "")
	if err == nil {
		t.Error("UpdateNodeCapability should return forced error")
	}
}
