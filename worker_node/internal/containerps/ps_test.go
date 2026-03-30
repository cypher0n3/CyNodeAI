package containerps

import "testing"

func TestNameListed_PrefixCollision(t *testing.T) {
	ps := "cynodeai-managed-pma-test\n"
	name := "cynodeai-managed-pma"
	if NameListed(ps, name) {
		t.Fatal("prefix of another container name must not match")
	}
}

func TestNameListed_ExactLine(t *testing.T) {
	ps := "other\ncynodeai-managed-pma\n"
	name := "cynodeai-managed-pma"
	if !NameListed(ps, name) {
		t.Fatal("exact line must match")
	}
}
