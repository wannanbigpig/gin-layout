package todo

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func newTodoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE user_todos (
		id integer primary key autoincrement,
		assignee_id integer not null default 0,
		creator_id integer not null default 0,
		source_type text not null default '',
		source_key text not null default '',
		title text not null default '',
		description text,
		priority text not null default 'normal',
		status text not null default 'pending',
		action_url text not null default '',
		due_at datetime,
		completed_at datetime,
		canceled_at datetime,
		created_at datetime,
		updated_at datetime,
		deleted_at integer not null default 0
	)`).Error; err != nil {
		t.Fatalf("create user_todos: %v", err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX user_todos_active_source_key
		ON user_todos(assignee_id, source_type, source_key)
		WHERE deleted_at = 0 AND status IN ('pending', 'processing')`).Error; err != nil {
		t.Fatalf("create active todo unique index: %v", err)
	}
	return db
}

func TestManualTodoOwnershipAndBusinessMutationBoundary(t *testing.T) {
	ctx := context.Background()
	svc := NewServiceWithDB(newTodoTestDB(t))
	manual, err := svc.CreateManual(ctx, 7, &form.UserTodoCreate{
		SourceKey: "personal-key",
		Title:     "My task",
		Priority:  model.TodoPriorityHigh,
	})
	if err != nil {
		t.Fatalf("create manual todo: %v", err)
	}
	if manual.AssigneeID != 7 || manual.CreatorID != 7 || manual.SourceType != model.TodoSourceManual {
		t.Fatalf("manual todo ownership mismatch: %#v", manual)
	}
	if _, err := svc.CreateManual(ctx, 7, &form.UserTodoCreate{SourceKey: "personal-key", Title: "Duplicate"}); err == nil {
		t.Fatal("expected duplicate manual source key to fail")
	}

	business, err := svc.UpsertBusinessTodo(ctx, BusinessTodoInput{
		AssigneeID: 7,
		CreatorID:  9,
		SourceType: "order_approval",
		SourceKey:  "order:1001",
		Title:      "Approve order",
		Priority:   model.TodoPriorityUrgent,
	})
	if err != nil {
		t.Fatalf("upsert business todo: %v", err)
	}
	if _, err := svc.UpdateStatus(ctx, 7, business.ID, model.TodoStatusCompleted); err == nil {
		t.Fatal("expected business todo status to be controlled by its source module")
	}
	if _, err := svc.UpsertBusinessTodo(ctx, BusinessTodoInput{
		AssigneeID: 7, SourceType: "MANUAL", SourceKey: "case-bypass", Title: "invalid",
	}); err == nil {
		t.Fatal("manual source type must be rejected case-insensitively")
	}
	if _, err := svc.CreateManual(ctx, 8, &form.UserTodoCreate{Title: "Other user"}); err != nil {
		t.Fatalf("create other user's todo: %v", err)
	}

	list, err := svc.List(ctx, 7, &form.UserTodoList{Paginate: form.Paginate{Page: 1, PerPage: 20}})
	if err != nil {
		t.Fatalf("list user todos: %v", err)
	}
	if list.Total != 2 || len(list.Data) != 2 {
		t.Fatalf("expected exactly current user's todos, total=%d data=%#v", list.Total, list.Data)
	}

	newTitle := "Changed"
	if _, err := svc.UpdateManual(ctx, 7, &form.UserTodoUpdate{ID: business.ID, Title: &newTitle}); err == nil {
		t.Fatal("expected business todo core fields to reject personal update")
	}
	if _, err := svc.UpdateManual(ctx, 8, &form.UserTodoUpdate{ID: manual.ID, Title: &newTitle}); err == nil {
		t.Fatal("expected other user mutation to be hidden")
	}
	if err := svc.DeleteManual(ctx, 7, business.ID); err == nil {
		t.Fatal("expected business todo delete to be rejected")
	}
	if err := svc.DeleteManual(ctx, 7, manual.ID); err != nil {
		t.Fatalf("delete own manual todo: %v", err)
	}
}

func TestTodoActionURLMustBeInternalPath(t *testing.T) {
	ctx := context.Background()
	svc := NewServiceWithDB(newTodoTestDB(t))
	for _, actionURL := range []string{"https://example.com", "//example.com", `\\example.com`, "javascript:alert(1)"} {
		if _, err := svc.CreateManual(ctx, 7, &form.UserTodoCreate{Title: "unsafe", ActionURL: actionURL}); err == nil {
			t.Fatalf("expected unsafe action URL %q to fail", actionURL)
		}
	}
	created, err := svc.CreateManual(ctx, 7, &form.UserTodoCreate{Title: "safe", ActionURL: "/order/list?id=1"})
	if err != nil || created.ActionURL != "/order/list?id=1" {
		t.Fatalf("expected internal action URL to pass: todo=%#v err=%v", created, err)
	}
}

func TestTodoStatusTransitionsAndBusinessIdempotency(t *testing.T) {
	ctx := context.Background()
	svc := NewServiceWithDB(newTodoTestDB(t))
	manual, err := svc.CreateManual(ctx, 7, &form.UserTodoCreate{Title: "Transition"})
	if err != nil {
		t.Fatalf("create manual todo: %v", err)
	}
	processing, err := svc.UpdateStatus(ctx, 7, manual.ID, model.TodoStatusProcessing)
	if err != nil || processing.Status != model.TodoStatusProcessing {
		t.Fatalf("pending -> processing failed: todo=%#v err=%v", processing, err)
	}
	completed, err := svc.UpdateStatus(ctx, 7, manual.ID, model.TodoStatusCompleted)
	if err != nil || completed.CompletedAt == nil || completed.CanceledAt != nil {
		t.Fatalf("processing -> completed failed: todo=%#v err=%v", completed, err)
	}
	if _, err := svc.UpdateStatus(ctx, 7, manual.ID, model.TodoStatusPending); err == nil {
		t.Fatal("terminal todo must not reopen through personal status API")
	}

	input := BusinessTodoInput{
		AssigneeID: 7,
		CreatorID:  9,
		SourceType: "contract",
		SourceKey:  "contract:42",
		Title:      "Review contract",
	}
	first, err := svc.UpsertBusinessTodo(ctx, input)
	if err != nil {
		t.Fatalf("first business upsert: %v", err)
	}
	input.Title = "Review updated contract"
	second, err := svc.UpsertBusinessTodo(ctx, input)
	if err != nil {
		t.Fatalf("second business upsert: %v", err)
	}
	if second.ID != first.ID || second.Title != input.Title {
		t.Fatalf("business upsert must reuse one row: first=%#v second=%#v", first, second)
	}
	closed, err := svc.CloseBusinessTodo(ctx, BusinessTodoCloseInput{
		AssigneeID: 7,
		SourceType: input.SourceType,
		SourceKey:  input.SourceKey,
		Status:     model.TodoStatusCanceled,
	})
	if err != nil || closed.CanceledAt == nil {
		t.Fatalf("close business todo: todo=%#v err=%v", closed, err)
	}
	again, err := svc.CloseBusinessTodo(ctx, BusinessTodoCloseInput{
		AssigneeID: 7,
		SourceType: input.SourceType,
		SourceKey:  input.SourceKey,
		Status:     model.TodoStatusCanceled,
	})
	if err != nil || again.ID != closed.ID {
		t.Fatalf("idempotent business close failed: todo=%#v err=%v", again, err)
	}
	unchanged, err := svc.UpsertBusinessTodo(ctx, input)
	if err != nil || unchanged.Status != model.TodoStatusCanceled || unchanged.CanceledAt == nil {
		t.Fatalf("business upsert retry must preserve terminal state: todo=%#v err=%v", unchanged, err)
	}
	input.StartNewCycle = true
	reopened, err := svc.UpsertBusinessTodo(ctx, input)
	if err != nil || reopened.ID == closed.ID || reopened.Status != model.TodoStatusPending {
		t.Fatalf("explicit new business cycle must create a new pending todo: old=%#v new=%#v err=%v", closed, reopened, err)
	}
}

func TestManualTodoSourceKeyCanBeReusedAfterSoftDelete(t *testing.T) {
	ctx := context.Background()
	svc := NewServiceWithDB(newTodoTestDB(t))
	create := func() *model.UserTodo {
		t.Helper()
		item, err := svc.CreateManual(ctx, 7, &form.UserTodoCreate{SourceKey: "reusable", Title: "Reusable"})
		if err != nil {
			t.Fatalf("create reusable todo: %v", err)
		}
		return item
	}
	first := create()
	if err := svc.DeleteManual(ctx, 7, first.ID); err != nil {
		t.Fatalf("delete first reusable todo: %v", err)
	}
	second := create()
	if err := svc.DeleteManual(ctx, 7, second.ID); err != nil {
		t.Fatalf("delete second reusable todo in same second: %v", err)
	}
}

func TestTodoSummaryAndDueScope(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	svc := NewServiceWithDB(newTodoTestDB(t))
	svc.now = func() time.Time { return now }
	ctx := context.Background()
	create := func(title, dueAt string) *model.UserTodo {
		t.Helper()
		item, err := svc.CreateManual(ctx, 7, &form.UserTodoCreate{Title: title, DueAt: dueAt})
		if err != nil {
			t.Fatalf("create %s: %v", title, err)
		}
		return item
	}
	overdue := create("overdue", "2026-09-16 09:00:00")
	today := create("today", "2026-09-17 18:00:00")
	processing := create("processing", "2026-09-18 09:00:00")
	create("no due", "")
	if _, err := svc.UpdateStatus(ctx, 7, processing.ID, model.TodoStatusProcessing); err != nil {
		t.Fatalf("mark processing: %v", err)
	}

	summary, err := svc.Summary(ctx, 7)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.Pending != 3 || summary.Processing != 1 || summary.Overdue != 1 || summary.DueToday != 1 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	for scope, expectedID := range map[string]uint{"overdue": overdue.ID, "today": today.ID} {
		list, err := svc.List(ctx, 7, &form.UserTodoList{
			Paginate: form.Paginate{Page: 1, PerPage: 20},
			DueScope: scope,
		})
		if err != nil {
			t.Fatalf("list scope %s: %v", scope, err)
		}
		if list.Total != 1 || len(list.Data) != 1 {
			t.Fatalf("scope %s result mismatch: %#v", scope, list)
		}
		item := list.Data[0].(*resources.UserTodoResources)
		if item.ID != expectedID {
			t.Fatalf("scope %s id=%d want=%d", scope, item.ID, expectedID)
		}
	}
}

func TestTodoListOrdersOverdueThenPriorityThenDueAt(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	svc := NewServiceWithDB(newTodoTestDB(t))
	svc.now = func() time.Time { return now }
	ctx := context.Background()
	create := func(title, priority, dueAt string) {
		t.Helper()
		if _, err := svc.CreateManual(ctx, 7, &form.UserTodoCreate{Title: title, Priority: priority, DueAt: dueAt}); err != nil {
			t.Fatalf("create %s: %v", title, err)
		}
	}
	create("future urgent", model.TodoPriorityUrgent, "2026-09-18 08:00:00")
	create("overdue low", model.TodoPriorityLow, "2026-09-16 08:00:00")
	create("overdue urgent later", model.TodoPriorityUrgent, "2026-09-16 10:00:00")
	create("overdue urgent earlier", model.TodoPriorityUrgent, "2026-09-16 09:00:00")
	completed, err := svc.CreateManual(ctx, 7, &form.UserTodoCreate{Title: "completed urgent", Priority: model.TodoPriorityUrgent})
	if err != nil {
		t.Fatalf("create completed urgent: %v", err)
	}
	if _, err := svc.UpdateStatus(ctx, 7, completed.ID, model.TodoStatusCompleted); err != nil {
		t.Fatalf("complete urgent: %v", err)
	}

	list, err := svc.List(ctx, 7, &form.UserTodoList{Paginate: form.Paginate{Page: 1, PerPage: 20}})
	if err != nil {
		t.Fatalf("list todos: %v", err)
	}
	titles := make([]string, 0, len(list.Data))
	for _, raw := range list.Data {
		titles = append(titles, raw.(*resources.UserTodoResources).Title)
	}
	want := []string{"overdue urgent earlier", "overdue urgent later", "overdue low", "future urgent", "completed urgent"}
	for index := range want {
		if titles[index] != want[index] {
			t.Fatalf("unexpected todo order: got=%v want=%v", titles, want)
		}
	}
}
