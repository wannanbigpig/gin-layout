package taskcenter

import (
	"context"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestRecorderFinalizesExpiredExecution(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status string
		event  string
	}{
		{"timeout", context.DeadlineExceeded, model.TaskRunStatusTimeout, model.TaskEventTimeout},
		{"interrupted", context.Canceled, model.TaskRunStatusInterrupted, model.TaskEventInterrupted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newTaskCenterTestDB(t)
			r := NewRecorderWithDB(db)
			run, err := r.Start(context.Background(), RunStart{TaskCode: "export:file-list", SourceID: "expired-task"})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer cancel()
			if err := r.Finish(ctx, run, RunFinish{Error: tc.err}); err != nil {
				t.Fatal(err)
			}
			var stored model.TaskRun
			if err := db.First(&stored, run.ID).Error; err != nil {
				t.Fatal(err)
			}
			if stored.Status != tc.status || stored.FinishedAt == nil {
				t.Fatalf("unfinished execution: %+v", stored)
			}
			var event model.TaskRunEvent
			if err := db.Where("run_id = ? AND event_type = ?", run.ID, tc.event).First(&event).Error; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRecorderTimeoutWithAutomaticRetryReusesRun(t *testing.T) {
	db := newTaskCenterTestDB(t)
	r := NewRecorderWithDB(db)
	run, err := r.Start(context.Background(), RunStart{TaskCode: "export:file-list", SourceID: "retry-timeout", Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Finish(ctx, run, RunFinish{Status: model.TaskRunStatusRetrying, Error: context.DeadlineExceeded}); err != nil {
		t.Fatal(err)
	}
	next, err := r.Start(context.Background(), RunStart{TaskCode: "export:file-list", SourceID: "retry-timeout", Attempt: 2})
	if err != nil {
		t.Fatal(err)
	}
	if next.ID != run.ID {
		t.Fatalf("retry split the execution: %d -> %d", run.ID, next.ID)
	}
}
