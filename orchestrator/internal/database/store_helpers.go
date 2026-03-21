// Package database: shared helpers for Store methods and record mapping (reduces duplicate patterns).
package database

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
)

// newGormModelUUIDNow returns a GormModelUUID with a new id and current UTC timestamps.
func newGormModelUUIDNow() gormmodel.GormModelUUID {
	now := time.Now().UTC()
	return gormmodel.GormModelUUID{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// getDomainByID loads a record by primary key and maps it with conv.
func getDomainByID[R any, M any](db *DB, ctx context.Context, id uuid.UUID, op string, conv func(*R) *M) (*M, error) {
	record, err := getByID[R](db, ctx, id, op)
	if err != nil {
		return nil, err
	}
	return conv(record), nil
}

// getDomainWhere loads the first record matching col=val and maps it with conv.
func getDomainWhere[R any, M any](db *DB, ctx context.Context, col string, val interface{}, op string, conv func(*R) *M) (*M, error) {
	record, err := getWhere[R](db, ctx, col, val, op)
	if err != nil {
		return nil, err
	}
	return conv(record), nil
}

// auditRow is implemented by audit GORM records that use ensureAuditIDAndTime on ID/CreatedAt.
type auditRow interface {
	auditIDPtr() (*uuid.UUID, *time.Time)
}

// auditGeneratedOut is implemented by domain audit models that receive generated id and created_at.
type auditGeneratedOut interface {
	SetGeneratedAuditIDs(id uuid.UUID, createdAt time.Time)
}

// insertAuditModel inserts an audit row and copies generated id/time into the domain model.
func insertAuditModel(db *DB, ctx context.Context, row auditRow, record interface{}, out auditGeneratedOut, op string) error {
	return db.persistAuditInsert(ctx, row, record, op, out.SetGeneratedAuditIDs)
}

// persistAuditInsert ensures id/time, inserts record, then syncs generated id/time into the caller model.
func (db *DB) persistAuditInsert(ctx context.Context, row auditRow, record interface{}, op string, sync func(id uuid.UUID, at time.Time)) error {
	idPtr, tPtr := row.auditIDPtr()
	ensureAuditIDAndTime(idPtr, tPtr)
	if err := db.createRecord(ctx, record, op); err != nil {
		return err
	}
	idPtr2, tPtr2 := row.auditIDPtr()
	sync(*idPtr2, *tPtr2)
	return nil
}
