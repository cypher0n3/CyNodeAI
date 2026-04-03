package handlers

import (
	"context"
	"crypto/rand"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/natsjwt"
)

// BumpPMAHostConfigVersion assigns a new ULID config_version on the worker node that hosts PMA managed services.
func BumpPMAHostConfigVersion(ctx context.Context, db database.Store, logger *slog.Logger) (string, error) {
	hostSlug := selectPMAHostNodeSlug(ctx, db, "", logger)
	if hostSlug == "" {
		return "", nil
	}
	nodes, err := db.ListActiveNodes(ctx)
	if err != nil {
		return "", err
	}
	var hostID uuid.UUID
	for _, n := range nodes {
		if n.NodeSlug == hostSlug {
			hostID = n.ID
			break
		}
	}
	if hostID == uuid.Nil {
		return "", nil
	}
	newVer := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
	if err := db.UpdateNodeConfigVersion(ctx, hostID, newVer); err != nil {
		return "", err
	}
	if js := getJetStreamForConfigBump(); js != nil {
		tid := natsjwt.DefaultTenantID
		nid := hostID.String()
		prod := natsutil.Producer{Service: "cynode-control-plane", InstanceID: nid}
		scope := natsutil.Scope{TenantID: tid, Sensitivity: "internal"}
		payload := &natsutil.NodeConfigChangedPayloadV1{
			NodeID:        nid,
			ConfigVersion: newVer,
			Ts:            time.Now().UTC().Format(time.RFC3339),
		}
		if err := natsutil.PublishConfigChanged(js, tid, nid, prod, scope, natsutil.Correlation{}, payload); err != nil && logger != nil {
			logger.Warn("nats config_changed publish", "error", err)
		}
	}
	return newVer, nil
}
