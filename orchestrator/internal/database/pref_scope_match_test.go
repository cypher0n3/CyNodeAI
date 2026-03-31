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

func TestMergePreferenceEntriesByScopeOrder_InvalidJSONSkipped(t *testing.T) {
	uid := uuid.New()
	scopes := []prefScope{{scopeType: "user", scopeID: &uid}}
	bad := "not-json"
	entries := []*models.PreferenceEntry{
		{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "user", ScopeID: &uid, Key: "k", Value: &bad}},
	}
	out := mergePreferenceEntriesByScopeOrder(scopes, entries)
	if len(out) != 0 {
		t.Fatalf("invalid JSON should be skipped: %v", out)
	}
}

func TestMergePreferenceEntriesByScopeOrder_ScopeOverrideOrder(t *testing.T) {
	uid := uuid.New()
	pid := uuid.New()
	scopes := []prefScope{
		{scopeType: "system", scopeID: nil},
		{scopeType: "user", scopeID: &uid},
		{scopeType: "project", scopeID: &pid},
	}
	s1 := `"from-system"`
	s2 := `"from-user"`
	s3 := `"from-project"`
	entries := []*models.PreferenceEntry{
		{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "system", ScopeID: nil, Key: "x", Value: &s1}},
		{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "user", ScopeID: &uid, Key: "x", Value: &s2}},
		{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "project", ScopeID: &pid, Key: "x", Value: &s3}},
	}
	out := mergePreferenceEntriesByScopeOrder(scopes, entries)
	if out["x"] != "from-project" {
		t.Fatalf("later scope should win: got %#v", out["x"])
	}
}

func TestMergePreferenceEntriesByScopeOrder_MultipleKeys(t *testing.T) {
	uid := uuid.New()
	scopes := []prefScope{{scopeType: "user", scopeID: &uid}}
	va := `"a"`
	vb := `"b"`
	entries := []*models.PreferenceEntry{
		{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "user", ScopeID: &uid, Key: "k1", Value: &va}},
		{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "user", ScopeID: &uid, Key: "k2", Value: &vb}},
	}
	out := mergePreferenceEntriesByScopeOrder(scopes, entries)
	if out["k1"] != "a" || out["k2"] != "b" {
		t.Fatalf("got %#v", out)
	}
}

func TestMergePreferenceEntriesByScopeOrder_JSONNumber(t *testing.T) {
	uid := uuid.New()
	scopes := []prefScope{{scopeType: "user", scopeID: &uid}}
	raw := "42"
	entries := []*models.PreferenceEntry{
		{PreferenceEntryBase: models.PreferenceEntryBase{ScopeType: "user", ScopeID: &uid, Key: "n", Value: &raw}},
	}
	out := mergePreferenceEntriesByScopeOrder(scopes, entries)
	fv, ok := out["n"].(float64)
	if !ok || fv != 42 {
		t.Fatalf("got %#v", out["n"])
	}
}
