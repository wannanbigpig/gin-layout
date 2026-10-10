package taskcenter

import (
	"context"
	"errors"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestCronRetryExecutesAndKeepsRetryChain(t *testing.T) {
	for _, outcome := range []string{"success", "handler failure", "canceled"} {
		t.Run(outcome, func(t *testing.T) {
			db := newTaskCenterTestDB(t)
			r := NewRecorderWithDB(db)
			defer SetRecorderForTesting(r)()
			original, err := r.Start(context.Background(), RunStart{TaskCode: "cron:test", Kind: model.TaskKindCron, Source: model.TaskSourceManual, Payload: []byte(`{"source":"original"}`)})
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Finish(context.Background(), original, RunFinish{Error: errors.New("original failure")}); err != nil {
				t.Fatal(err)
			}
			defer setTaskRunLoaderForTest(func(id uint) (*model.TaskRun, error) {
				var run model.TaskRun
				err := db.First(&run, id).Error
				return &run, err
			})()
			defer setTaskDefinitionLoaderForTest(func(code string) (*model.TaskDefinition, error) {
				return &model.TaskDefinition{Code: code, Kind: model.TaskKindCron, Handler: "test-handler", Status: 1, AllowRetry: 1}, nil
			})()
			publisher := &stubPublisher{err: errors.New("cron must not enter Asynq")}
			defer queue.SetPublisherForTesting(publisher)()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			svc := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db})
			called := 0
			defer setCronExecutorForTest(func(ctx context.Context, handler string, payload map[string]any) error {
				called++
				if handler != "test-handler" || payload["source"] != "original" {
					t.Fatalf("wrong execution: %s %v", handler, payload)
				}
				if _, err := svc.RetryTask(context.Background(), original.ID, 9, "tester"); err == nil {
					t.Fatal("in-flight cron retry must block another retry")
				}
				if outcome == "handler failure" {
					return errors.New("retry failure")
				}
				if outcome == "canceled" {
					cancel()
					return ctx.Err()
				}
				return nil
			})()
			result, err := svc.RetryTask(ctx, original.ID, 9, "tester")
			if outcome != "success" && err == nil {
				t.Fatal("expected handler failure")
			}
			if outcome == "success" && err != nil {
				t.Fatal(err)
			}
			if called != 1 || publisher.job != nil {
				t.Fatalf("cron execution calls=%d queue=%+v", called, publisher.job)
			}
			var retry model.TaskRun
			if err := db.Where("retry_of_run_id = ?", original.ID).First(&retry).Error; err != nil {
				t.Fatal(err)
			}
			want := model.TaskRunStatusSuccess
			if outcome == "handler failure" {
				want = model.TaskRunStatusFailed
			}
			if outcome == "canceled" {
				want = model.TaskRunStatusInterrupted
			}
			if retry.Kind != model.TaskKindCron || retry.Status != want || retry.Queue != "" || retry.RetryRootRunID != original.ID || retry.RetrySeq != 1 || retry.TriggerUserID != 9 {
				t.Fatalf("bad retry: %+v", retry)
			}
			if outcome == "success" && (result["run_id"] != retry.ID || result["queue"] != "") {
				t.Fatalf("bad response %v", result)
			}
			if _, err := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db}).RetryTask(context.Background(), original.ID, 9, "tester"); err == nil {
				t.Fatal("resolved/exhausted chain must reject another retry")
			}
		})
	}
}

func TestHighRiskCronCannotEnableRetry(t *testing.T) {
	db := newTaskCenterTestDB(t)
	definition := &model.TaskDefinition{
		Code: "cron:risky", Kind: model.TaskKindCron, Handler: "risky", CronSpec: "0 0 * * * *",
		Status: 1, AllowManual: 1, IsHighRisk: model.TaskHighRisk,
	}
	if err := db.Create(definition).Error; err != nil {
		t.Fatal(err)
	}
	defer setTaskDefinitionLoaderForTest(func(string) (*model.TaskDefinition, error) { return definition, nil })()
	_, err := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db}).UpdateTaskOperationConfig(context.Background(), &form.TaskOperationConfigForm{
		TaskCode: definition.Code, Status: 1, AllowManual: 1, AllowRetry: 1, CronSpec: definition.CronSpec,
	})
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) || businessErr.GetCode() != e.InvalidParameter {
		t.Fatalf("expected confirmed-trigger guidance, got %v", err)
	}
	var stored model.TaskDefinition
	if err := db.First(&stored, definition.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.AllowRetry != 0 || stored.AllowManual != 1 {
		t.Fatalf("high-risk operation config changed: %+v", stored)
	}
}

func TestHighRiskCronRetryRequiresConfirmedTrigger(t *testing.T) {
	run := &model.TaskRun{BaseModel: model.BaseModel{ID: 1}, Kind: model.TaskKindCron, Status: model.TaskRunStatusFailed, TaskCode: "cron:risky"}
	definition := &model.TaskDefinition{Kind: model.TaskKindCron, Status: 1, AllowRetry: 1, IsHighRisk: 1, Handler: "risky"}
	if allowed, _ := evaluateTaskRunRetryState(run, definition, retryChainSummary{}); allowed {
		t.Fatal("high-risk cron needs confirmed trigger")
	}
	defer setTaskRunLoaderForTest(func(uint) (*model.TaskRun, error) { return run, nil })()
	defer setTaskDefinitionLoaderForTest(func(string) (*model.TaskDefinition, error) { return definition, nil })()
	_, err := NewTaskCenterService().RetryTask(context.Background(), 1, 9, "tester")
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) || businessErr.GetCode() != e.InvalidParameter {
		t.Fatalf("expected confirmed-trigger guidance, got %v", err)
	}
}
