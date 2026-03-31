package database

import (
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func TestPrefScopeMatches(t *testing.T) {
	uid := uuid.New()
	pid := uuid.New()
	tid := uuid.New()
	cases := []struct {
		name string
		s    prefScope
		e    *models.PreferenceEntry
		want bool
	}{
		{
			"system_nil_entry",
			prefScope{scopeType: "system", scopeID: nil},
			&models.PreferenceEntry{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "system", ScopeID: nil}},
			true,
		},
		{
			"system_mismatch_scope_id",
			prefScope{scopeType: "system", scopeID: nil},
			&models.PreferenceEntry{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "system", ScopeID: &uid}},
			false,
		},
		{
			"user_match",
			prefScope{scopeType: "user", scopeID: &uid},
			&models.PreferenceEntry{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "user", ScopeID: &uid}},
			true,
		},
		{
			"user_wrong_id",
			prefScope{scopeType: "user", scopeID: &uid},
			&models.PreferenceEntry{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "user", ScopeID: &pid}},
			false,
		},
		{
			"user_entry_nil_scope_id",
			prefScope{scopeType: "user", scopeID: &uid},
			&models.PreferenceEntry{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "user", ScopeID: nil}},
			false,
		},
		{
			"type_mismatch",
			prefScope{scopeType: "project", scopeID: &pid},
			&models.PreferenceEntry{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "user", ScopeID: &uid}},
			false,
		},
		{
			"task_match",
			prefScope{scopeType: "task", scopeID: &tid},
			&models.PreferenceEntry{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "task", ScopeID: &tid}},
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := prefScopeMatches(c.s, c.e); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
