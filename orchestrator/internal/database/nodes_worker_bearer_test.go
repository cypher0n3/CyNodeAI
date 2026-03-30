package database

import (
	"encoding/base64"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/fieldcrypt"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func TestNodeRecordToDomain_DecryptsWorkerBearer(t *testing.T) {
	key := fieldcrypt.DeriveKeyFromJWTSecret("jwt-for-unit-test")
	db := &DB{workerBearerKey: key}
	enc, err := fieldcrypt.EncryptWorkerBearerToken("secret-token", key)
	if err != nil {
		t.Fatal(err)
	}
	r := &NodeRecord{
		GormModelUUID: gormmodel.GormModelUUID{ID: uuid.New()},
		NodeBase: models.NodeBase{
			NodeSlug:             "n1",
			Status:               models.NodeStatusActive,
			WorkerAPIBearerToken: &enc,
		},
	}
	n, err := db.nodeRecordToDomain(r)
	if err != nil {
		t.Fatal(err)
	}
	if n.WorkerAPIBearerToken == nil || *n.WorkerAPIBearerToken != "secret-token" {
		t.Fatalf("got %v", n.WorkerAPIBearerToken)
	}
}

func TestNodeRecordToDomain_NoKeyPassesPlaintext(t *testing.T) {
	db := &DB{}
	plain := "still-plain"
	r := &NodeRecord{
		GormModelUUID: gormmodel.GormModelUUID{ID: uuid.New()},
		NodeBase: models.NodeBase{
			NodeSlug:             "n2",
			Status:               models.NodeStatusActive,
			WorkerAPIBearerToken: &plain,
		},
	}
	n, err := db.nodeRecordToDomain(r)
	if err != nil {
		t.Fatal(err)
	}
	if n.WorkerAPIBearerToken == nil || *n.WorkerAPIBearerToken != plain {
		t.Fatalf("got %v", n.WorkerAPIBearerToken)
	}
}

func TestNodeRecordToDomain_BadEnc1ReturnsError(t *testing.T) {
	key := fieldcrypt.DeriveKeyFromJWTSecret("jwt-for-unit-test")
	db := &DB{workerBearerKey: key}
	bad := fieldcrypt.EncryptedWorkerBearerPrefix + base64.StdEncoding.EncodeToString([]byte{1, 2, 3})
	r := &NodeRecord{
		GormModelUUID: gormmodel.GormModelUUID{ID: uuid.New()},
		NodeBase: models.NodeBase{
			NodeSlug:             "n3",
			Status:               models.NodeStatusActive,
			WorkerAPIBearerToken: &bad,
		},
	}
	_, err := db.nodeRecordToDomain(r)
	if err == nil {
		t.Fatal("expected decrypt error")
	}
}
