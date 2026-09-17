package model

import "testing"

func TestUserTodoModelContract(t *testing.T) {
	if got := NewUserTodo().TableName(); got != "user_todos" {
		t.Fatalf("unexpected table name: %s", got)
	}
	statuses := []string{TodoStatusPending, TodoStatusProcessing, TodoStatusCompleted, TodoStatusCanceled}
	for _, status := range statuses {
		if status == "" {
			t.Fatal("todo status constants must not be empty")
		}
	}
	if TodoSourceManual != "manual" || TodoPriorityNormal != "normal" {
		t.Fatalf("unexpected todo defaults: source=%s priority=%s", TodoSourceManual, TodoPriorityNormal)
	}
}
