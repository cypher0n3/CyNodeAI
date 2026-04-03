package handlers

import (
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/google/uuid"
)

func TestPoolTargetSlotCount(t *testing.T) {
	t.Setenv("PMA_WARM_POOL_MIN_FREE", "1")
	t.Setenv("PMA_WARM_POOL_MAX_SLOTS", "8")
	if n := poolTargetSlotCount(0); n != 1 {
		t.Fatalf("0 sessions + min free => 1, got %d", n)
	}
	if n := poolTargetSlotCount(1); n != 2 {
		t.Fatalf("1 session + min free => 2, got %d", n)
	}
	t.Setenv("PMA_WARM_POOL_MAX_SLOTS", "2")
	if n := poolTargetSlotCount(5); n != 2 {
		t.Fatalf("capped by max slots, got %d", n)
	}
}

func TestParsePMAPoolSlot(t *testing.T) {
	if s, ok := parsePMAPoolSlot("pma-pool-3"); !ok || s != 3 {
		t.Fatalf("parse pma-pool-3: ok=%v s=%d", ok, s)
	}
	if _, ok := parsePMAPoolSlot("pma-sb-abc"); ok {
		t.Fatal("expected legacy id to not parse as pool")
	}
}

func TestFirstFreePMAPoolSlot(t *testing.T) {
	used := map[int]struct{}{0: {}, 2: {}}
	if g := firstFreePMAPoolSlot(used, 4); g != 1 {
		t.Fatalf("got %d want 1", g)
	}
	if g := firstFreePMAPoolSlot(map[int]struct{}{0: {}, 1: {}}, 2); g != -1 {
		t.Fatalf("got %d want -1", g)
	}
}

func TestPmaUsedSlotsInTargetRange(t *testing.T) {
	b := []*models.SessionBinding{
		{SessionBindingBase: models.SessionBindingBase{ServiceID: poolServiceID(0)}},
		{SessionBindingBase: models.SessionBindingBase{ServiceID: "pma-sb-x"}},
		nil,
	}
	got := pmaUsedSlotsInTargetRange(b, 2)
	if len(got) != 1 {
		t.Fatalf("got %v want one slot", got)
	}
}

func TestPmaGreedyHasBindingForSession(t *testing.T) {
	sid := uuid.New()
	valid := []*models.SessionBinding{{SessionBindingBase: models.SessionBindingBase{SessionID: sid}}}
	if !pmaGreedyHasBindingForSession(valid, sid) {
		t.Fatal("expected true")
	}
	if pmaGreedyHasBindingForSession(valid, uuid.New()) {
		t.Fatal("expected false")
	}
}

func TestPmaGreedyExistingOrUsedSlots_SkipsOutOfRangeSlot(t *testing.T) {
	sid := uuid.New()
	b := []*models.SessionBinding{
		{SessionBindingBase: models.SessionBindingBase{SessionID: sid, ServiceID: poolServiceID(10)}},
	}
	existing, used := pmaGreedyExistingOrUsedSlots(b, 2, sid)
	if existing != "" || len(used) != 0 {
		t.Fatalf("existing=%q used=%v", existing, used)
	}
}

func TestFirstFreePMAPoolSlot_ZeroTarget(t *testing.T) {
	if g := firstFreePMAPoolSlot(nil, 0); g != -1 {
		t.Fatalf("got %d want -1", g)
	}
}

func TestPickPMAPoolServiceIDForBindings_KeepsWhenWorkerReady(t *testing.T) {
	sid := uuid.New()
	valid := []*models.SessionBinding{
		{SessionBindingBase: models.SessionBindingBase{SessionID: sid, ServiceID: poolServiceID(0)}},
	}
	got, err := pickPMAPoolServiceIDForBindings(2, sid, valid, []string{poolServiceID(0)}, nil)
	if err != nil || got != poolServiceID(0) {
		t.Fatalf("got %q err=%v want %s", got, err, poolServiceID(0))
	}
}

func TestPickPMAPoolServiceIDForBindings_RebindsWhenStale(t *testing.T) {
	sid := uuid.New()
	other := uuid.New()
	valid := []*models.SessionBinding{
		{SessionBindingBase: models.SessionBindingBase{SessionID: sid, ServiceID: poolServiceID(0)}},
		{SessionBindingBase: models.SessionBindingBase{SessionID: other, ServiceID: poolServiceID(1)}},
	}
	got, err := pickPMAPoolServiceIDForBindings(4, sid, valid, []string{poolServiceID(2)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != poolServiceID(2) {
		t.Fatalf("got %q want %s", got, poolServiceID(2))
	}
}

func TestPmaGreedyExistingOrUsedSlots(t *testing.T) {
	sid := uuid.New()
	other := uuid.New()
	b := []*models.SessionBinding{
		{SessionBindingBase: models.SessionBindingBase{SessionID: sid, ServiceID: poolServiceID(0)}},
		{SessionBindingBase: models.SessionBindingBase{SessionID: other, ServiceID: poolServiceID(1)}},
	}
	existing, used := pmaGreedyExistingOrUsedSlots(b, 3, sid)
	if existing != poolServiceID(0) || used != nil {
		t.Fatalf("existing=%q used=%v", existing, used)
	}
	wantOther := uuid.New()
	_, used = pmaGreedyExistingOrUsedSlots(b, 3, wantOther)
	if len(used) != 2 {
		t.Fatalf("used=%v want 2 slots", used)
	}
}
