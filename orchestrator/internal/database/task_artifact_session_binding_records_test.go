package database

import (
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/google/uuid"
)

func TestGormUUIDTimestamps(t *testing.T) {
	id := uuid.MustParse("00000000-0000-4000-8000-000000000030")
	ts := time.Unix(100, 0).UTC()
	m := &gormmodel.GormModelUUID{ID: id, CreatedAt: ts, UpdatedAt: ts}
	gotID, ca, ua := gormUUIDTimestamps(m)
	if gotID != id || !ca.Equal(ts) || !ua.Equal(ts) {
		t.Fatalf("gormUUIDTimestamps = %v %v %v", gotID, ca, ua)
	}
}
