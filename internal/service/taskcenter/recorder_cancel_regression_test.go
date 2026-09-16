package taskcenter

import (
	"context"
	"errors"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/queue"
)

func TestRecorderCancelsRetryingAndProtectsTerminalStates(t *testing.T) {
	db := newTaskCenterTestDB(t)
	r := NewRecorderWithDB(db)
	ctx := context.Background()
	run, err := r.Start(ctx, RunStart{TaskCode: "retry:test", Kind: model.TaskKindAsync, Source: model.TaskSourceQueue, SourceID: "retry-task", Queue: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Finish(ctx, run, RunFinish{Status: model.TaskRunStatusRetrying}); err != nil {
		t.Fatal(err)
	}
	if err := r.Finish(ctx, run, RunFinish{Status: model.TaskRunStatusSuccess}); !errors.Is(err, ErrTaskRunStatusChanged) {
		t.Fatalf("retrying must not become successful without Start: %v", err)
	}
	if err := r.Finish(ctx, run, RunFinish{Status: model.TaskRunStatusCanceled, CancelReason: "user canceled"}); err != nil {
		t.Fatal(err)
	}
	var stored model.TaskRun
	if err := db.First(&stored, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.TaskRunStatusCanceled {
		t.Fatal(stored.Status)
	}
	if err := r.Finish(ctx, run, RunFinish{Status: model.TaskRunStatusCanceled}); !errors.Is(err, ErrTaskRunStatusChanged) {
		t.Fatalf("double cancel must fail: %v", err)
	}
	var stat model.TaskRunStat
	if err := db.Where("task_code = ?", "retry:test").First(&stat).Error; err != nil {
		t.Fatal(err)
	}
	if stat.TotalCount != 1 || stat.CanceledCount != 1 {
		t.Fatalf("unexpected cancellation stats: %+v", stat)
	}
}

func TestCancelRetryingTaskUpdatesQueueAndStoredRun(t *testing.T) {
	db := newTaskCenterTestDB(t)
	recorder := NewRecorderWithDB(db)
	ctx := context.Background()
	run, err := recorder.Start(ctx, RunStart{TaskCode: "retry:cancel", Kind: model.TaskKindAsync, Source: model.TaskSourceQueue, SourceID: "retry-id", Queue: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if err := recorder.Finish(ctx, run, RunFinish{Status: model.TaskRunStatusRetrying}); err != nil {
		t.Fatal(err)
	}
	defer setTaskRunLoaderForTest(func(id uint) (*model.TaskRun, error) {
		var stored model.TaskRun
		err := db.First(&stored, id).Error
		return &stored, err
	})()
	defer SetRecorderForTesting(recorder)()
	inspector := &stubInspector{}
	defer queue.SetInspectorForTesting(inspector)()
	defer setExportTaskServiceForTesting(&stubExportTaskService{})()
	if _, err := NewTaskCenterService().CancelTask(ctx, run.ID, 1, "tester", "manual cancel"); err != nil {
		t.Fatal(err)
	}
	if len(inspector.deleted) != 1 || inspector.deleted[0].taskID != "retry-id" {
		t.Fatalf("queue task was not removed: %+v", inspector.deleted)
	}
	var stored model.TaskRun
	if err := db.First(&stored, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.TaskRunStatusCanceled {
		t.Fatal(stored.Status)
	}
	var event model.TaskRunEvent
	if err := db.Where("run_id = ? AND event_type = ?", run.ID, model.TaskEventCancel).First(&event).Error; err != nil {
		t.Fatal(err)
	}
}
