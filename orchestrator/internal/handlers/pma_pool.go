// Package handlers: PMA warm-pool sizing and pool slot assignment (REQ-ORCHES-0192).
package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/readiness"
)

var pmaPoolSlotRe = regexp.MustCompile(`^pma-pool-(\d+)$`)

func pmaPoolMinFreeSlots() int {
	return parsePositiveIntEnv("PMA_WARM_POOL_MIN_FREE", 1)
}

func pmaPoolMaxSlots() int {
	return parsePositiveIntEnv("PMA_WARM_POOL_MAX_SLOTS", 16)
}

func parsePositiveIntEnv(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// poolTargetSlotCount returns how many pma-pool-* slots the orchestrator should keep in desired state.
// activeSessionsWithValidRefresh is the count of interactive sessions that currently hold an active binding
// with a non-expired refresh session (one PMA slot each when assigned).
func poolTargetSlotCount(activeSessionsWithValidRefresh int) int {
	maxS := pmaPoolMaxSlots()
	minFree := pmaPoolMinFreeSlots()
	need := activeSessionsWithValidRefresh + minFree
	if need < 1 {
		need = 1
	}
	if need > maxS {
		return maxS
	}
	return need
}

func poolServiceID(slot int) string {
	return fmt.Sprintf("pma-pool-%d", slot)
}

func parsePMAPoolSlot(serviceID string) (int, bool) {
	m := pmaPoolSlotRe.FindStringSubmatch(strings.TrimSpace(serviceID))
	if len(m) != 2 {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// collectActiveBindingsWithValidRefresh returns session_bindings in active state whose refresh session exists and is usable.
func collectActiveBindingsWithValidRefresh(ctx context.Context, db database.Store) ([]*models.SessionBinding, error) {
	all, err := db.ListAllActiveSessionBindings(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var out []*models.SessionBinding
	for _, b := range all {
		if b == nil {
			continue
		}
		rs, err := db.GetRefreshSessionByID(ctx, b.SessionID)
		if err != nil {
			continue
		}
		if !rs.IsActive || rs.ExpiresAt.Before(now) {
			continue
		}
		out = append(out, b)
	}
	return out, nil
}

func pmaUsedSlotsInTargetRange(bindings []*models.SessionBinding, targetSlots int) map[int]struct{} {
	used := make(map[int]struct{})
	for _, b := range bindings {
		if b == nil {
			continue
		}
		if slot, ok := parsePMAPoolSlot(b.ServiceID); ok && slot < targetSlots {
			used[slot] = struct{}{}
		}
	}
	return used
}

func firstFreePMAPoolSlot(used map[int]struct{}, targetSlots int) int {
	for s := 0; s < targetSlots; s++ {
		if _, taken := used[s]; !taken {
			return s
		}
	}
	return -1
}

// migrateLegacyPMAPoolServiceIDs assigns pma-pool-* service_ids to bindings that still use legacy ids (e.g. pma-sb-*)
// or pool slots outside the current target range.
func migrateLegacyPMAPoolServiceIDs(ctx context.Context, db database.Store, bindings []*models.SessionBinding, targetSlots int, logger *slog.Logger) error {
	used := pmaUsedSlotsInTargetRange(bindings, targetSlots)
	for _, b := range bindings {
		if b == nil {
			continue
		}
		if slot, ok := parsePMAPoolSlot(b.ServiceID); ok && slot < targetSlots {
			continue
		}
		free := firstFreePMAPoolSlot(used, targetSlots)
		if free < 0 {
			if logger != nil {
				logger.Warn("pma warm pool: no free slot for legacy binding migration",
					"binding_key", b.BindingKey, "target_slots", targetSlots)
			}
			return fmt.Errorf("pma warm pool: no free slot for binding %s", b.BindingKey)
		}
		lineage := models.SessionBindingLineage{UserID: b.UserID, SessionID: b.SessionID, ThreadID: b.ThreadID}
		newID := poolServiceID(free)
		if _, err := db.UpsertSessionBinding(ctx, lineage, newID, models.SessionBindingStateActive); err != nil {
			return err
		}
		b.ServiceID = newID
		used[free] = struct{}{}
	}
	return nil
}

// pmaGreedyExistingOrUsedSlots returns the pool id already assigned to interactiveSessionID, or a map of taken slots.
func pmaGreedyExistingOrUsedSlots(valid []*models.SessionBinding, target int, interactiveSessionID uuid.UUID) (existingID string, used map[int]struct{}) {
	used = make(map[int]struct{})
	for _, b := range valid {
		slot, ok := parsePMAPoolSlot(b.ServiceID)
		if !ok || slot >= target {
			continue
		}
		if b.SessionID == interactiveSessionID {
			return b.ServiceID, nil
		}
		used[slot] = struct{}{}
	}
	return "", used
}

// pickPMAPoolServiceIDForGreedy chooses the pool service_id for an interactive session during greedy provisioning.
func pickPMAPoolServiceIDForGreedy(ctx context.Context, db database.Store, userID, interactiveSessionID uuid.UUID, logger *slog.Logger) (string, error) {
	now := time.Now().UTC()
	rs, err := db.GetRefreshSessionByID(ctx, interactiveSessionID)
	if err != nil {
		return "", err
	}
	if !rs.IsActive || rs.ExpiresAt.Before(now) {
		return "", fmt.Errorf("refresh session inactive or expired")
	}
	valid, err := collectActiveBindingsWithValidRefresh(ctx, db)
	if err != nil {
		return "", err
	}
	nSess := len(valid)
	if !pmaGreedyHasBindingForSession(valid, interactiveSessionID) {
		nSess++
	}
	target := poolTargetSlotCount(nSess)
	if err := migrateLegacyPMAPoolServiceIDs(ctx, db, valid, target, logger); err != nil {
		return "", err
	}
	valid, err = collectActiveBindingsWithValidRefresh(ctx, db)
	if err != nil {
		return "", err
	}
	readyIDs, err := readiness.ReadyManagedPMAServiceIDs(ctx, db)
	if err != nil {
		return "", err
	}
	return pickPMAPoolServiceIDForBindings(target, interactiveSessionID, valid, readyIDs, logger)
}

func pmaPoolReadySet(readyServiceIDs []string) map[string]struct{} {
	readySet := make(map[string]struct{}, len(readyServiceIDs))
	for _, id := range readyServiceIDs {
		readySet[strings.TrimSpace(id)] = struct{}{}
	}
	return readySet
}

// pmaPoolOccupancy returns this session's current pool service_id (if any) and slots taken by other sessions.
func pmaPoolOccupancy(valid []*models.SessionBinding, target int, interactiveSessionID uuid.UUID) (staleSID string, used map[int]struct{}) {
	used = make(map[int]struct{})
	for _, b := range valid {
		if b == nil {
			continue
		}
		slot, ok := parsePMAPoolSlot(b.ServiceID)
		if !ok || slot >= target {
			continue
		}
		if b.SessionID == interactiveSessionID {
			staleSID = strings.TrimSpace(b.ServiceID)
			continue
		}
		used[slot] = struct{}{}
	}
	return staleSID, used
}

func pmaPoolStaleSlotIndex(staleSID string) int {
	if staleSID == "" {
		return -1
	}
	s, ok := parsePMAPoolSlot(staleSID)
	if !ok {
		return -1
	}
	return s
}

// pickFreePMAPoolSlot returns the first free slot; when requireReady is true, only slots in readySet qualify.
func pickFreePMAPoolSlot(target int, used map[int]struct{}, staleSlot int, readySet map[string]struct{}, requireReady bool) (string, bool) {
	for s := 0; s < target; s++ {
		if _, taken := used[s]; taken {
			continue
		}
		if s == staleSlot {
			continue
		}
		psid := poolServiceID(s)
		if requireReady {
			if _, ok := readySet[psid]; ok {
				return psid, true
			}
			continue
		}
		return psid, true
	}
	return "", false
}

// pickPMAPoolServiceIDForBindings assigns a warm-pool slot for interactiveSessionID.
// If the session already has a binding, it is kept only when that service_id is still
// reported ready in worker capability snapshots; otherwise a different free slot is chosen
// (preferring slots the worker currently reports as ready).
func pickPMAPoolServiceIDForBindings(
	target int,
	interactiveSessionID uuid.UUID,
	valid []*models.SessionBinding,
	readyServiceIDs []string,
	logger *slog.Logger,
) (string, error) {
	readySet := pmaPoolReadySet(readyServiceIDs)
	staleSID, used := pmaPoolOccupancy(valid, target, interactiveSessionID)
	if staleSID != "" {
		if _, ok := readySet[staleSID]; ok {
			return staleSID, nil
		}
		if logger != nil {
			logger.Warn(
				"pma greedy: session binding service_id not reported ready by worker; re-picking pool slot",
				"interactive_session_id", interactiveSessionID,
				"stale_service_id", staleSID,
				"ready_pma_count", len(readyServiceIDs),
			)
		}
	}
	staleSlot := pmaPoolStaleSlotIndex(staleSID)
	if psid, ok := pickFreePMAPoolSlot(target, used, staleSlot, readySet, true); ok {
		return psid, nil
	}
	if psid, ok := pickFreePMAPoolSlot(target, used, staleSlot, readySet, false); ok {
		return psid, nil
	}
	if staleSlot >= 0 {
		if _, taken := used[staleSlot]; !taken {
			return poolServiceID(staleSlot), nil
		}
	}
	return "", fmt.Errorf("pma warm pool exhausted (target=%d)", target)
}

func pmaGreedyHasBindingForSession(valid []*models.SessionBinding, interactiveSessionID uuid.UUID) bool {
	for _, b := range valid {
		if b.SessionID == interactiveSessionID {
			return true
		}
	}
	return false
}
