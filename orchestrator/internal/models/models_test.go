package models

import (
	"testing"
)

func TestTaskStatusConstants(t *testing.T) {
	statuses := []string{
		TaskStatusPending,
		TaskStatusRunning,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusCancelled,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("Task status constant is empty")
		}
	}
}

func TestJobStatusConstants(t *testing.T) {
	statuses := []string{
		JobStatusQueued,
		JobStatusRunning,
		JobStatusCompleted,
		JobStatusFailed,
		JobStatusCancelled,
		JobStatusLeaseExpired,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("Job status constant is empty")
		}
	}
}

func TestNodeStatusConstants(t *testing.T) {
	statuses := []string{
		NodeStatusRegistered,
		NodeStatusActive,
		NodeStatusInactive,
		NodeStatusDrained,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("Node status constant is empty")
		}
	}
}

func TestUser_Fields(t *testing.T) {
	user := User{
		Handle:   "testuser",
		IsActive: true,
	}

	if user.Handle != "testuser" {
		t.Errorf("Handle = %s, want testuser", user.Handle)
	}

	if !user.IsActive {
		t.Error("IsActive should be true")
	}
}

func TestTask_Fields(t *testing.T) {
	prompt := "test prompt"
	task := Task{
		Status: TaskStatusPending,
		Prompt: &prompt,
	}

	if task.Status != TaskStatusPending {
		t.Errorf("Status = %s, want %s", task.Status, TaskStatusPending)
	}

	if *task.Prompt != "test prompt" {
		t.Errorf("Prompt = %s, want test prompt", *task.Prompt)
	}
}

func TestJob_Fields(t *testing.T) {
	job := Job{
		Status: JobStatusQueued,
	}

	if job.Status != JobStatusQueued {
		t.Errorf("Status = %s, want %s", job.Status, JobStatusQueued)
	}
}

func TestNode_Fields(t *testing.T) {
	node := Node{
		NodeSlug: "test-node",
		Status:   NodeStatusRegistered,
	}

	if node.NodeSlug != "test-node" {
		t.Errorf("NodeSlug = %s, want test-node", node.NodeSlug)
	}

	if node.Status != NodeStatusRegistered {
		t.Errorf("Status = %s, want %s", node.Status, NodeStatusRegistered)
	}
}
