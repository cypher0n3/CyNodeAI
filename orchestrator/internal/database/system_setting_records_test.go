package database

import (
	"testing"
	"time"
)

func TestSystemSettingRecord_ToSystemSetting_nilReceiver(t *testing.T) {
	t.Parallel()
	var r *SystemSettingRecord
	if got := r.ToSystemSetting(); got != nil {
		t.Fatalf("got %+v", got)
	}
}

func TestSystemSettingRecord_ToSystemSetting_roundTrip(t *testing.T) {
	t.Parallel()
	ts := time.Now().UTC()
	r := &SystemSettingRecord{
		Key:       "k",
		Value:     ptr("v"),
		ValueType: "string",
		Version:   2,
		UpdatedAt: ts,
		UpdatedBy: ptr("u"),
	}
	got := r.ToSystemSetting()
	if got.Key != "k" || got.Version != 2 || got.UpdatedAt != ts {
		t.Fatalf("got %+v", got)
	}
}

func ptr(s string) *string { return &s }
