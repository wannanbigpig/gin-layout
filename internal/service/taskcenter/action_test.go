package taskcenter

import (
	"context"
	stderrors "errors"
	"testing"

	taskcron "github.com/wannanbigpig/gin-layout/internal/cron"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type stubPublisher struct {
	info queue.JobInfo
	err  error
}

func (s *stubPublisher) Enqueue(ctx context.Context, job queue.Job) (queue.JobInfo, error) {
	_ = ctx
	_ = job
	if s.err != nil {
		return queue.JobInfo{}, s.err
	}
	return s.info, nil
}

type fakeActionRecorder struct {
	enqueueInputs []RunStart
	startInputs   []RunStart
	finishInputs  []RunFinish
}

type stubInspector struct {
	deleteTaskErr error
	cancelErr     error
	deleted       []struct {
		queue  string
		taskID string
	}
	canceled []string
}

func (f *fakeActionRecorder) Enqueue(ctx context.Context, input RunStart) (*model.TaskRun, error) {
	_ = ctx
	f.enqueueInputs = append(f.enqueueInputs, input)
	return &model.TaskRun{BaseModel: model.BaseModel{ID: uint(len(f.enqueueInputs))}, TaskCode: input.TaskCode}, nil
}

func (f *fakeActionRecorder) Start(ctx context.Context, input RunStart) (*model.TaskRun, error) {
	_ = ctx
	f.startInputs = append(f.startInputs, input)
	return &model.TaskRun{BaseModel: model.BaseModel{ID: uint(len(f.startInputs))}, TaskCode: input.TaskCode}, nil
}

func (f *fakeActionRecorder) Finish(ctx context.Context, run *model.TaskRun, input RunFinish) error {
	_ = ctx
	_ = run
	f.finishInputs = append(f.finishInputs, input)
	return nil
}

func (s *stubInspector) DeleteTask(ctx context.Context, queueName, taskID string) error {
	_ = ctx
	s.deleted = append(s.deleted, struct {
		queue  string
		taskID string
	}{queue: queueName, taskID: taskID})
	return s.deleteTaskErr
}

func (s *stubInspector) CancelProcessing(ctx context.Context, taskID string) error {
	_ = ctx
	s.canceled = append(s.canceled, taskID)
	return s.cancelErr
}

func TestTriggerTaskSuccess(t *testing.T) {
	restoreTaskDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:        taskCode,
			Kind:        model.TaskKindAsync,
			Status:      1,
			AllowManual: 1,
		}, nil
	})
	defer restoreTaskDefinition()

	restorePublisher := queue.SetPublisherForTesting(&stubPublisher{
		info: queue.JobInfo{ID: "task-1", Queue: "default", Type: "demo:send"},
	})
	defer restorePublisher()

	fakeRecorder := &fakeActionRecorder{}
	restoreRecorder := SetRecorderForTesting(fakeRecorder)
	defer restoreRecorder()

	svc := NewTaskCenterService()
	result, err := svc.TriggerTask(context.Background(), &form.TaskTriggerForm{
		TaskCode: "demo:send",
		Queue:    "default",
		Payload:  map[string]any{"name": "codex"},
	}, 1, "tester")
	if err != nil {
		t.Fatalf("TriggerTask returned error: %v", err)
	}

	if result["task_id"] != "task-1" {
		t.Fatalf("unexpected task id: %#v", result["task_id"])
	}
	if len(fakeRecorder.enqueueInputs) != 1 {
		t.Fatalf("expected enqueue record called once, got %d", len(fakeRecorder.enqueueInputs))
	}
	if len(fakeRecorder.finishInputs) != 0 {
		t.Fatalf("did not expect finish record on success, got %d", len(fakeRecorder.finishInputs))
	}
}

func TestTriggerTaskReturnsDependencyNotReadyWhenPublisherUnavailable(t *testing.T) {
	restoreTaskDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:        taskCode,
			Kind:        model.TaskKindAsync,
			Status:      1,
			AllowManual: 1,
		}, nil
	})
	defer restoreTaskDefinition()

	restorePublisher := queue.SetPublisherForTesting(nil)
	defer restorePublisher()

	fakeRecorder := &fakeActionRecorder{}
	restoreRecorder := SetRecorderForTesting(fakeRecorder)
	defer restoreRecorder()

	svc := NewTaskCenterService()
	_, err := svc.TriggerTask(context.Background(), &form.TaskTriggerForm{
		TaskCode: "demo:send",
		Queue:    "default",
	}, 1, "tester")
	if err == nil {
		t.Fatal("expected error when publisher unavailable")
	}

	var be *e.BusinessError
	if !stderrors.As(err, &be) {
		t.Fatalf("expected business error, got %T", err)
	}
	if be.GetCode() != e.ServiceDependencyNotReady {
		t.Fatalf("expected code %d, got %d", e.ServiceDependencyNotReady, be.GetCode())
	}
	if len(fakeRecorder.finishInputs) != 1 {
		t.Fatalf("expected finish record called once, got %d", len(fakeRecorder.finishInputs))
	}
}

func TestTriggerTaskCronSuccess(t *testing.T) {
	restoreTaskDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:        taskCode,
			Kind:        model.TaskKindCron,
			Handler:     taskcron.HandlerCronDemo,
			Status:      1,
			AllowManual: 1,
		}, nil
	})
	defer restoreTaskDefinition()

	var calledHandler string
	restoreExecutor := setCronExecutorForTest(func(ctx context.Context, handler string, payload map[string]any) error {
		_ = ctx
		_ = payload
		calledHandler = handler
		return nil
	})
	defer restoreExecutor()

	fakeRecorder := &fakeActionRecorder{}
	restoreRecorder := SetRecorderForTesting(fakeRecorder)
	defer restoreRecorder()

	svc := NewTaskCenterService()
	result, err := svc.TriggerTask(context.Background(), &form.TaskTriggerForm{
		TaskCode: "cron:demo",
		Payload:  map[string]any{"source": "test"},
	}, 1, "tester")
	if err != nil {
		t.Fatalf("TriggerTask returned error: %v", err)
	}

	if calledHandler != taskcron.HandlerCronDemo {
		t.Fatalf("unexpected cron handler: %s", calledHandler)
	}
	if result["run_id"] != uint(1) {
		t.Fatalf("unexpected run_id: %#v", result["run_id"])
	}
	if len(fakeRecorder.startInputs) != 1 {
		t.Fatalf("expected start record called once, got %d", len(fakeRecorder.startInputs))
	}
	if len(fakeRecorder.finishInputs) != 1 {
		t.Fatalf("expected finish record called once, got %d", len(fakeRecorder.finishInputs))
	}
	if fakeRecorder.finishInputs[0].Error != nil {
		t.Fatalf("expected cron run success, got error: %v", fakeRecorder.finishInputs[0].Error)
	}
}

func TestTriggerTaskReturnsErrorWhenManualNotAllowed(t *testing.T) {
	restoreTaskDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:        taskCode,
			Kind:        model.TaskKindCron,
			Status:      1,
			AllowManual: 0,
		}, nil
	})
	defer restoreTaskDefinition()

	svc := NewTaskCenterService()
	_, err := svc.TriggerTask(context.Background(), &form.TaskTriggerForm{
		TaskCode: "cron:demo",
	}, 1, "tester")
	if err == nil {
		t.Fatal("expected error when task does not allow manual trigger")
	}

	var be *e.BusinessError
	if !stderrors.As(err, &be) {
		t.Fatalf("expected business error, got %T", err)
	}
	if be.GetCode() != e.InvalidParameter {
		t.Fatalf("expected code %d, got %d", e.InvalidParameter, be.GetCode())
	}
}

func TestTriggerTaskHighRiskRequiresConfirm(t *testing.T) {
	restoreTaskDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:        taskCode,
			Kind:        model.TaskKindAsync,
			Status:      1,
			AllowManual: 1,
			IsHighRisk:  model.TaskHighRisk,
		}, nil
	})
	defer restoreTaskDefinition()

	svc := NewTaskCenterService()
	_, err := svc.TriggerTask(context.Background(), &form.TaskTriggerForm{
		TaskCode: "demo:send",
	}, 1, "tester")
	if err == nil {
		t.Fatal("expected error when high-risk task confirm is empty")
	}

	var be *e.BusinessError
	if !stderrors.As(err, &be) {
		t.Fatalf("expected business error, got %T", err)
	}
	if be.GetCode() != e.InvalidParameter {
		t.Fatalf("expected code %d, got %d", e.InvalidParameter, be.GetCode())
	}
}

func TestTriggerTaskRecordsConfirmAndReason(t *testing.T) {
	restoreTaskDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:        taskCode,
			Kind:        model.TaskKindAsync,
			Status:      1,
			AllowManual: 1,
			IsHighRisk:  model.TaskHighRisk,
		}, nil
	})
	defer restoreTaskDefinition()

	restorePublisher := queue.SetPublisherForTesting(&stubPublisher{
		info: queue.JobInfo{ID: "task-1", Queue: "default", Type: "demo:send"},
	})
	defer restorePublisher()

	fakeRecorder := &fakeActionRecorder{}
	restoreRecorder := SetRecorderForTesting(fakeRecorder)
	defer restoreRecorder()

	svc := NewTaskCenterService()
	_, err := svc.TriggerTask(context.Background(), &form.TaskTriggerForm{
		TaskCode: "demo:send",
		Confirm:  "CONFIRM",
		Reason:   "manual high-risk operation",
	}, 1, "tester")
	if err != nil {
		t.Fatalf("TriggerTask returned error: %v", err)
	}

	if len(fakeRecorder.enqueueInputs) != 1 {
		t.Fatalf("expected enqueue record called once, got %d", len(fakeRecorder.enqueueInputs))
	}
	if fakeRecorder.enqueueInputs[0].TriggerConfirm != "CONFIRM" || fakeRecorder.enqueueInputs[0].TriggerReason != "manual high-risk operation" {
		t.Fatalf("unexpected trigger audit meta: %#v", fakeRecorder.enqueueInputs[0])
	}
}

func TestRetryTaskRespectsCurrentDefinition(t *testing.T) {
	restoreRun := setTaskRunLoaderForTest(func(runID uint) (*model.TaskRun, error) {
		return &model.TaskRun{
			BaseModel: model.BaseModel{ID: runID},
			TaskCode:  "demo:send",
			Kind:      model.TaskKindAsync,
			Queue:     "default",
			Status:    model.TaskRunStatusFailed,
			Payload:   `{"name":"codex"}`,
		}, nil
	})
	defer restoreRun()

	restoreDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:       taskCode,
			Kind:       model.TaskKindAsync,
			Status:     model.TaskStatusEnabled,
			AllowRetry: 0,
		}, nil
	})
	defer restoreDefinition()

	svc := NewTaskCenterService()
	_, err := svc.RetryTask(context.Background(), 101, 1, "tester")
	if err == nil {
		t.Fatal("expected error when current definition disallows retry")
	}

	var be *e.BusinessError
	if !stderrors.As(err, &be) {
		t.Fatalf("expected business error, got %T", err)
	}
	if be.GetCode() != e.InvalidParameter {
		t.Fatalf("expected code %d, got %d", e.InvalidParameter, be.GetCode())
	}
}

func TestCancelTaskDeletesPendingTaskAndMarksRunCanceled(t *testing.T) {
	restoreLoader := setTaskRunLoaderForTest(func(runID uint) (*model.TaskRun, error) {
		return &model.TaskRun{
			BaseModel: model.BaseModel{ID: runID},
			TaskCode:  "demo:send",
			Kind:      model.TaskKindAsync,
			SourceID:  "task-1",
			Queue:     "default",
			Status:    model.TaskRunStatusPending,
		}, nil
	})
	defer restoreLoader()

	fakeRecorder := &fakeActionRecorder{}
	restoreRecorder := SetRecorderForTesting(fakeRecorder)
	defer restoreRecorder()

	inspector := &stubInspector{}
	restoreInspector := queue.SetInspectorForTesting(inspector)
	defer restoreInspector()

	svc := NewTaskCenterService()
	result, err := svc.CancelTask(context.Background(), 101, 1, "tester", "manual cancel")
	if err != nil {
		t.Fatalf("CancelTask returned error: %v", err)
	}
	if result["status"] != model.TaskRunStatusCanceled {
		t.Fatalf("unexpected cancel status: %#v", result["status"])
	}
	if len(inspector.deleted) != 1 || inspector.deleted[0].taskID != "task-1" {
		t.Fatalf("unexpected deleted tasks: %#v", inspector.deleted)
	}
	if len(fakeRecorder.finishInputs) != 1 || fakeRecorder.finishInputs[0].Status != model.TaskRunStatusCanceled {
		t.Fatalf("expected recorder finish with canceled status, got %#v", fakeRecorder.finishInputs)
	}
	if fakeRecorder.finishInputs[0].CanceledBy != 1 || fakeRecorder.finishInputs[0].CanceledByAccount != "tester" || fakeRecorder.finishInputs[0].CancelReason != "manual cancel" {
		t.Fatalf("unexpected cancel meta: %#v", fakeRecorder.finishInputs[0])
	}
}

func TestCancelTaskReturnsDependencyNotReadyWhenInspectorUnavailable(t *testing.T) {
	restoreLoader := setTaskRunLoaderForTest(func(runID uint) (*model.TaskRun, error) {
		return &model.TaskRun{
			BaseModel: model.BaseModel{ID: runID},
			TaskCode:  "demo:send",
			Kind:      model.TaskKindAsync,
			SourceID:  "task-1",
			Queue:     "default",
			Status:    model.TaskRunStatusPending,
		}, nil
	})
	defer restoreLoader()

	restoreInspector := queue.SetInspectorForTesting(nil)
	defer restoreInspector()

	svc := NewTaskCenterService()
	_, err := svc.CancelTask(context.Background(), 101, 1, "tester", "")
	if err == nil {
		t.Fatal("expected error when inspector unavailable")
	}

	var be *e.BusinessError
	if !stderrors.As(err, &be) {
		t.Fatalf("expected business error, got %T", err)
	}
	if be.GetCode() != e.ServiceDependencyNotReady {
		t.Fatalf("expected code %d, got %d", e.ServiceDependencyNotReady, be.GetCode())
	}
}

func setTaskRunLoaderForTest(loader func(runID uint) (*model.TaskRun, error)) func() {
	previous := loadTaskRunByID
	loadTaskRunByID = loader
	return func() {
		loadTaskRunByID = previous
	}
}

func setTaskDefinitionLoaderForTest(loader func(taskCode string) (*model.TaskDefinition, error)) func() {
	previous := loadTaskDefinitionByCode
	loadTaskDefinitionByCode = loader
	return func() {
		loadTaskDefinitionByCode = previous
	}
}

func setCronExecutorForTest(executor func(ctx context.Context, handler string, payload map[string]any) error) func() {
	previous := executeCronHandler
	executeCronHandler = executor
	return func() {
		executeCronHandler = previous
	}
}
