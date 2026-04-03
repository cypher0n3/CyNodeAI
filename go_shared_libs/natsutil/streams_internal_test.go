package natsutil

import "testing"

func TestStreamCoversSubjects(t *testing.T) {
	t.Parallel()
	have := []string{"cynode.session.activity.*.*", "cynode.session.attached.*.*"}
	need := []string{"cynode.session.activity.*.*"}
	if !streamCoversSubjects(have, need) {
		t.Fatal("expected true")
	}
	if streamCoversSubjects(have, []string{"cynode.session.activity.*.*", "cynode.node.config_changed.*.*"}) {
		t.Fatal("expected false when subject missing")
	}
}

func TestMergeSubjectLists(t *testing.T) {
	t.Parallel()
	out := mergeSubjectLists([]string{"a"}, []string{"a", "b"})
	if len(out) != 2 {
		t.Fatalf("got %#v", out)
	}
}
