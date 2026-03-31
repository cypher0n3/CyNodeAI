package telemetry

import (
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestGORMIndexes_TelemetryHotColumns(t *testing.T) {
	t.Parallel()
	parse := func(t *testing.T, dst interface{}) *schema.Schema {
		t.Helper()
		s, err := schema.Parse(dst, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		return s
	}
	cases := []struct {
		dst    interface{}
		fields []string
	}{
		{&ContainerInventory{}, []string{"Status", "LastSeenAt"}},
		{&LogEvent{}, []string{"OccurredAt", "ContainerID"}},
	}
	for _, tc := range cases {
		s := parse(t, tc.dst)
		for _, name := range tc.fields {
			f := s.LookUpField(name)
			if f == nil {
				t.Fatalf("field %s missing", name)
			}
			if !strings.Contains(f.Tag.Get("gorm"), "index") {
				t.Errorf("field %s: want gorm index in tag %q", name, f.Tag.Get("gorm"))
			}
		}
	}
}
