package artifacts

import (
	"testing"

	"github.com/google/uuid"
)

func TestStrArg_intArg_uuidArg_helpers(t *testing.T) {
	t.Parallel()
	if strArg(nil, "x") != "" {
		t.Fatal("strArg nil map")
	}
	if intArg(nil, "x") != 0 {
		t.Fatal("intArg nil map")
	}
	if uuidArg(nil, "x") != nil {
		t.Fatal("uuidArg nil map")
	}
	if intArg(map[string]interface{}{"x": "bad"}, "x") != 0 {
		t.Fatal("intArg wrong type")
	}
	if intArg(map[string]interface{}{"n": float64(7)}, "n") != 7 {
		t.Fatal("intArg float64")
	}
	if intArg(map[string]interface{}{"n": 9}, "n") != 9 {
		t.Fatal("intArg int")
	}
	if intArg(map[string]interface{}{"n": int32(1)}, "n") != 0 {
		t.Fatal("intArg int32 default")
	}
	uid := uuid.New()
	s := uid.String()
	if got := uuidArg(map[string]interface{}{"k": s}, "k"); got == nil || *got != uid {
		t.Fatal("uuidArg")
	}
	if uuidArg(map[string]interface{}{"k": "not-a-uuid"}, "k") != nil {
		t.Fatal("uuidArg bad string")
	}
}
