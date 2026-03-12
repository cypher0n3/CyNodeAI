package sba

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
)

func TestBuildInitialTodo(t *testing.T) {
	spec := &sbajob.JobSpec{
		Context: &sbajob.ContextSpec{
			Requirements:       []string{"R1", "R2"},
			AcceptanceCriteria: []string{"AC1"},
		},
		Steps: []sbajob.StepSpec{{Type: "run_command"}, {Type: "write_file"}},
	}
	items := BuildInitialTodo(spec)
	if len(items) != 5 {
		t.Fatalf("len(items) = %d, want 5", len(items))
	}
	if items[0].Title != "R1" || items[1].Title != "R2" {
		t.Errorf("requirements: %q", items[0].Title)
	}
	if items[2].Title != "Acceptance: AC1" {
		t.Errorf("acceptance: %q", items[2].Title)
	}
	if items[3].Title != "Step 1: run_command" || items[4].Title != "Step 2: write_file" {
		t.Errorf("steps: %q, %q", items[3].Title, items[4].Title)
	}
}

func TestWriteTodo(t *testing.T) {
	dir := t.TempDir()
	items := []TodoItem{{Title: "x", Done: true}}
	WriteTodo(dir, items)
	path := filepath.Join(dir, "todo.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got []TodoItem
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "x" || !got[0].Done {
		t.Errorf("got %+v", got)
	}
}

func TestWriteTodo_EmptyDir_NoOp(t *testing.T) {
	WriteTodo("", []TodoItem{{Title: "x"}})
}
