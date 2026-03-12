// Package sba provides todo list build and persist under /job/ per spec.
package sba

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
)

// TodoItem is a single todo entry (e.g. requirement or suggested step).
type TodoItem struct {
	Title string `json:"title"`
	Done  bool   `json:"done,omitempty"`
}

// BuildInitialTodo returns todo items from job context (requirements, acceptance criteria, suggested steps).
func BuildInitialTodo(spec *sbajob.JobSpec) []TodoItem {
	var items []TodoItem
	if spec.Context != nil {
		for _, r := range spec.Context.Requirements {
			items = append(items, TodoItem{Title: r})
		}
		for _, ac := range spec.Context.AcceptanceCriteria {
			items = append(items, TodoItem{Title: "Acceptance: " + ac})
		}
	}
	for i, s := range spec.Steps {
		items = append(items, TodoItem{Title: fmt.Sprintf("Step %d: %s", i+1, s.Type)})
	}
	return items
}

// WriteTodo persists todo items to jobDir/todo.json (best-effort).
func WriteTodo(jobDir string, items []TodoItem) {
	if jobDir == "" {
		return
	}
	path := filepath.Join(jobDir, "todo.json")
	raw, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o644)
}
