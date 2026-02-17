package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

type getterNotFoundCase struct {
	name    string
	queryRe string
	args    []any
	run     func(ctx context.Context, db *DB, args []any) error
}

var getterNotFoundCases = []getterNotFoundCase{
	{"GetUserByHandle", `SELECT .* FROM users WHERE handle`, []any{"nonexistent"},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetUserByHandle(ctx, args[0].(string))
			return err
		}},
	{"GetUserByID", `SELECT .* FROM users WHERE id`, []any{uuid.New()},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetUserByID(ctx, args[0].(uuid.UUID))
			return err
		}},
	{"GetPasswordCredentialByUserID", `SELECT .* FROM password_credentials WHERE user_id`, []any{uuid.New()},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetPasswordCredentialByUserID(ctx, args[0].(uuid.UUID))
			return err
		}},
	{"GetNodeBySlug", `SELECT .* FROM nodes WHERE node_slug`, []any{"nonexistent"},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetNodeBySlug(ctx, args[0].(string))
			return err
		}},
	{"GetNodeByID", `SELECT .* FROM nodes WHERE id`, []any{uuid.New()},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetNodeByID(ctx, args[0].(uuid.UUID))
			return err
		}},
	{"GetTaskByID", `SELECT .* FROM tasks WHERE id`, []any{uuid.New()},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetTaskByID(ctx, args[0].(uuid.UUID))
			return err
		}},
	{"GetJobByID", `SELECT .* FROM jobs WHERE id`, []any{uuid.New()},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetJobByID(ctx, args[0].(uuid.UUID))
			return err
		}},
}

func TestGettersNotFound(t *testing.T) {
	for _, tc := range getterNotFoundCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := setupDB(t)
			mock.ExpectQuery(tc.queryRe).WithArgs(toDriverValues(tc.args)...).WillReturnError(sql.ErrNoRows)
			err := tc.run(context.Background(), db, tc.args)
			assertErrNotFound(t, err)
		})
	}
}

func toDriverValues(args []any) []driver.Value {
	out := make([]driver.Value, len(args))
	for i := range args {
		out[i] = args[i]
	}
	return out
}

type getterErrorCase struct {
	name    string
	queryRe string
	args    []any
	run     func(ctx context.Context, db *DB, args []any) error
}

var getterErrorCases = []getterErrorCase{
	{"GetUserByHandle", `SELECT .* FROM users WHERE handle`, []any{"user"},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetUserByHandle(ctx, args[0].(string))
			return err
		}},
	{"GetPasswordCredentialByUserID", `SELECT .* FROM password_credentials`, []any{uuid.New()},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetPasswordCredentialByUserID(ctx, args[0].(uuid.UUID))
			return err
		}},
	{"GetNodeBySlug", `SELECT .* FROM nodes WHERE node_slug`, []any{"test-node"},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetNodeBySlug(ctx, args[0].(string))
			return err
		}},
	{"GetNodeByID", `SELECT .* FROM nodes WHERE id`, []any{uuid.New()},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetNodeByID(ctx, args[0].(uuid.UUID))
			return err
		}},
	{"GetTaskByID", `SELECT .* FROM tasks WHERE id`, []any{uuid.New()},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetTaskByID(ctx, args[0].(uuid.UUID))
			return err
		}},
	{"GetJobByID", `SELECT .* FROM jobs WHERE id`, []any{uuid.New()},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetJobByID(ctx, args[0].(uuid.UUID))
			return err
		}},
	{"GetNextQueuedJob", `SELECT .* FROM jobs WHERE status`, nil,
		func(ctx context.Context, db *DB, _ []any) error { _, err := db.GetNextQueuedJob(ctx); return err }},
	{"GetActiveRefreshSession", `SELECT .* FROM refresh_sessions`, []any{[]byte("hash")},
		func(ctx context.Context, db *DB, args []any) error {
			_, err := db.GetActiveRefreshSession(ctx, args[0].([]byte))
			return err
		}},
}

func TestQueryGettersError(t *testing.T) {
	for _, tc := range getterErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := setupDB(t)
			exp := mock.ExpectQuery(tc.queryRe)
			if tc.args != nil {
				exp.WithArgs(toDriverValues(tc.args)...)
			}
			exp.WillReturnError(errors.New("db error"))
			err := tc.run(context.Background(), db, tc.args)
			assertError(t, err)
		})
	}
}

func TestExecErrors(t *testing.T) {
	sessionID := uuid.New()
	userID := uuid.New()
	nodeID := uuid.New()
	taskID := uuid.New()
	jobID := uuid.New()

	cases := []struct {
		name   string
		execRe string
		run    func(ctx context.Context, db *DB) error
	}{
		{"InvalidateRefreshSession", `UPDATE refresh_sessions`,
			func(ctx context.Context, db *DB) error { return db.InvalidateRefreshSession(ctx, sessionID) }},
		{"InvalidateAllUserSessions", `UPDATE refresh_sessions`,
			func(ctx context.Context, db *DB) error { return db.InvalidateAllUserSessions(ctx, userID) }},
		{"UpdateNodeLastSeen", `UPDATE nodes SET last_seen_at`,
			func(ctx context.Context, db *DB) error { return db.UpdateNodeLastSeen(ctx, nodeID) }},
		{"UpdateNodeCapability", `UPDATE nodes SET capability_hash`,
			func(ctx context.Context, db *DB) error { return db.UpdateNodeCapability(ctx, nodeID, "hash") }},
		{"UpdateTaskStatus", `UPDATE tasks SET status`,
			func(ctx context.Context, db *DB) error {
				return db.UpdateTaskStatus(ctx, taskID, models.TaskStatusCompleted)
			}},
		{"UpdateTaskSummary", `UPDATE tasks SET summary`,
			func(ctx context.Context, db *DB) error { return db.UpdateTaskSummary(ctx, taskID, "summary") }},
		{"UpdateJobStatus", `UPDATE jobs SET status`,
			func(ctx context.Context, db *DB) error {
				return db.UpdateJobStatus(ctx, jobID, models.JobStatusRunning)
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := setupDB(t)
			mock.ExpectExec(tc.execRe).WillReturnError(errors.New("db error"))
			err := tc.run(context.Background(), db)
			assertError(t, err)
		})
	}
}

func TestListQueriesError(t *testing.T) {
	cases := []struct {
		name    string
		queryRe string
		args    []any
		run     func(ctx context.Context, db *DB) error
	}{
		{"ListActiveNodes", `SELECT .* FROM nodes WHERE status`, []any{models.NodeStatusActive},
			func(ctx context.Context, db *DB) error { _, err := db.ListActiveNodes(ctx); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := setupDB(t)
			mock.ExpectQuery(tc.queryRe).WithArgs(toDriverValues(tc.args)...).WillReturnError(errors.New("db error"))
			err := tc.run(context.Background(), db)
			assertError(t, err)
		})
	}
}

func TestExecUpdateSuccess(t *testing.T) {
	sessionID := uuid.New()
	userID := uuid.New()
	nodeID := uuid.New()
	taskID := uuid.New()
	jobID := uuid.New()

	cases := []struct {
		name   string
		execRe string
		args   []any
		run    func(ctx context.Context, db *DB) error
	}{
		{"InvalidateRefreshSession", `UPDATE refresh_sessions SET is_active`, []any{sessionID, sqlmock.AnyArg()},
			func(ctx context.Context, db *DB) error { return db.InvalidateRefreshSession(ctx, sessionID) }},
		{"InvalidateAllUserSessions", `UPDATE refresh_sessions SET is_active`, []any{userID, sqlmock.AnyArg()},
			func(ctx context.Context, db *DB) error { return db.InvalidateAllUserSessions(ctx, userID) }},
		{"UpdateNodeStatus", `UPDATE nodes SET status`, []any{nodeID, models.NodeStatusActive, sqlmock.AnyArg()},
			func(ctx context.Context, db *DB) error {
				return db.UpdateNodeStatus(ctx, nodeID, models.NodeStatusActive)
			}},
		{"UpdateNodeLastSeen", `UPDATE nodes SET last_seen_at`, []any{nodeID, sqlmock.AnyArg()},
			func(ctx context.Context, db *DB) error { return db.UpdateNodeLastSeen(ctx, nodeID) }},
		{"UpdateNodeCapability", `UPDATE nodes SET capability_hash`, []any{nodeID, "hash", sqlmock.AnyArg()},
			func(ctx context.Context, db *DB) error { return db.UpdateNodeCapability(ctx, nodeID, "hash") }},
		{"UpdateTaskStatus", `UPDATE tasks SET status`, []any{taskID, models.TaskStatusCompleted, sqlmock.AnyArg()},
			func(ctx context.Context, db *DB) error {
				return db.UpdateTaskStatus(ctx, taskID, models.TaskStatusCompleted)
			}},
		{"UpdateTaskSummary", `UPDATE tasks SET summary`, []any{taskID, "summary", sqlmock.AnyArg()},
			func(ctx context.Context, db *DB) error { return db.UpdateTaskSummary(ctx, taskID, "summary") }},
		{"UpdateJobStatus", `UPDATE jobs SET status`, []any{jobID, models.JobStatusRunning, sqlmock.AnyArg()},
			func(ctx context.Context, db *DB) error {
				return db.UpdateJobStatus(ctx, jobID, models.JobStatusRunning)
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := setupDB(t)
			mock.ExpectExec(tc.execRe).WithArgs(toDriverValues(tc.args)...).WillReturnResult(sqlmock.NewResult(0, 1))
			err := tc.run(context.Background(), db)
			if err != nil {
				t.Fatalf("%s failed: %v", tc.name, err)
			}
		})
	}
}
