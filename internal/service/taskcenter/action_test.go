package taskcenter

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	taskcron "github.com/wannanbigpig/gin-layout/internal/cron"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	baseservice "github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type stubPublisher struct {
	info queue.JobInfo
	err  error
	job  queue.Job
}

func (s *stubPublisher) Enqueue(ctx context.Context, job queue.Job) (queue.JobInfo, error) {
	_ = ctx
	s.job = job
	if s.err != nil {
		return queue.JobInfo{}, s.err
	}
	return s.info, nil
}

type stubExportTaskService struct {
	createRecordMatched bool
	createRecordErr     error
	createRecordInputs  []baseservice.ExportTaskBootstrapInput
	hasRecordReturn     bool
	hasRecordErr        error
	hasRecordInputs     []uint
	reassignReturn      bool
	reassignErr         error
	reassignInputs      []struct {
		oldTaskRunID uint
		newTaskRunID uint
		queueName    string
	}
	markFailedInputs []struct {
		taskRunID uint
		taskErr   error
	}
	markCanceledInputs []struct {
		taskRunID    uint
		cancelReason string
	}
}

func (s *stubExportTaskService) CreateRecordForTask(ctx context.Context, input baseservice.ExportTaskBootstrapInput) (*model.SysExportRecord, bool, error) {
	_ = ctx
	s.createRecordInputs = append(s.createRecordInputs, input)
	if s.createRecordErr != nil {
		return nil, false, s.createRecordErr
	}
	return &model.SysExportRecord{}, s.createRecordMatched, nil
}

func (s *stubExportTaskService) HasRecordForTaskRun(ctx context.Context, taskRunID uint) (bool, error) {
	_ = ctx
	s.hasRecordInputs = append(s.hasRecordInputs, taskRunID)
	if s.hasRecordErr != nil {
		return false, s.hasRecordErr
	}
	return s.hasRecordReturn, nil
}

func (s *stubExportTaskService) ReassignTaskRun(ctx context.Context, oldTaskRunID, newTaskRunID uint, queueName string) (bool, error) {
	_ = ctx
	s.reassignInputs = append(s.reassignInputs, struct {
		oldTaskRunID uint
		newTaskRunID uint
		queueName    string
	}{oldTaskRunID: oldTaskRunID, newTaskRunID: newTaskRunID, queueName: queueName})
	if s.reassignErr != nil {
		return false, s.reassignErr
	}
	return s.reassignReturn, nil
}

func (s *stubExportTaskService) MarkFailed(ctx context.Context, taskRunID uint, taskErr error) error {
	_ = ctx
	s.markFailedInputs = append(s.markFailedInputs, struct {
		taskRunID uint
		taskErr   error
	}{taskRunID: taskRunID, taskErr: taskErr})
	return nil
}

func (s *stubExportTaskService) MarkCanceledIfExists(ctx context.Context, taskRunID uint, cancelReason string) error {
	_ = ctx
	s.markCanceledInputs = append(s.markCanceledInputs, struct {
		taskRunID    uint
		cancelReason string
	}{taskRunID: taskRunID, cancelReason: cancelReason})
	return nil
}

func setExportTaskServiceForTesting(svc exportTaskService) func() {
	prev := newExportTaskService
	newExportTaskService = func() exportTaskService { return svc }
	return func() {
		newExportTaskService = prev
	}
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
	return &model.TaskRun{
		BaseModel:      model.BaseModel{ID: uint(len(f.enqueueInputs))},
		TaskCode:       input.TaskCode,
		RetryOfRunID:   input.RetryOfRunID,
		RetryRootRunID: input.RetryRootRunID,
		RetrySeq:       input.RetrySeq,
	}, nil
}

func (f *fakeActionRecorder) Start(ctx context.Context, input RunStart) (*model.TaskRun, error) {
	_ = ctx
	f.startInputs = append(f.startInputs, input)
	return &model.TaskRun{
		BaseModel:      model.BaseModel{ID: uint(len(f.startInputs))},
		TaskCode:       input.TaskCode,
		RetryOfRunID:   input.RetryOfRunID,
		RetryRootRunID: input.RetryRootRunID,
		RetrySeq:       input.RetrySeq,
	}, nil
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

func (s *stubInspector) GetQueueStats(ctx context.Context, queueName string) (*queue.QueueStats, error) {
	_ = ctx
	_ = queueName
	return &queue.QueueStats{}, nil
}

func (s *stubInspector) GetAllQueueStats(ctx context.Context) ([]*queue.QueueStats, error) {
	_ = ctx
	return []*queue.QueueStats{}, nil
}

func TestUpdateTaskOperationConfigUpdatesDefinition(t *testing.T) {
	db := newTaskCenterTestDB(t)
	insertTaskDefinition(t, db, "task:config", model.TaskRecordSuccessAll)
	notified := make([]string, 0, 1)
	restoreNotify := setTaskDefinitionChangeNotifierForTest(func(taskCode string) error {
		notified = append(notified, taskCode)
		return nil
	})
	defer restoreNotify()
	restoreDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:        taskCode,
			Kind:        model.TaskKindCron,
			CronSpec:    "0 0 3 * * *",
			Status:      model.TaskStatusEnabled,
			AllowManual: model.TaskManualNotAllowed,
			AllowRetry:  model.TaskRetryAllowed,
		}, nil
	})
	defer restoreDefinition()

	result, err := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db}).UpdateTaskOperationConfig(context.Background(), &form.TaskOperationConfigForm{
		TaskCode:    "task:config",
		Status:      model.TaskStatusDisabled,
		AllowManual: model.TaskManualAllowed,
		AllowRetry:  model.TaskRetryNotAllowed,
		CronSpec:    "0 0 4 * * *",
	})
	if err != nil {
		t.Fatalf("UpdateTaskOperationConfig returned error: %v", err)
	}

	resource, ok := result.(*resources.TaskDefinitionResources)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	if resource.Status != model.TaskStatusDisabled || resource.AllowManual != model.TaskManualAllowed || resource.AllowRetry != model.TaskRetryNotAllowed || resource.CronSpec != "0 0 4 * * *" {
		t.Fatalf("unexpected resource: %#v", resource)
	}

	var stored model.TaskDefinition
	if err := db.Where("code = ?", "task:config").First(&stored).Error; err != nil {
		t.Fatalf("query stored definition failed: %v", err)
	}
	if stored.Status != model.TaskStatusDisabled || stored.AllowManual != model.TaskManualAllowed || stored.AllowRetry != model.TaskRetryNotAllowed || stored.CronSpec != "0 0 4 * * *" {
		t.Fatalf("unexpected stored definition: status=%d allow_manual=%d allow_retry=%d cron_spec=%s", stored.Status, stored.AllowManual, stored.AllowRetry, stored.CronSpec)
	}
	if len(notified) != 1 || notified[0] != "task:config" {
		t.Fatalf("expected task definition change notification, got %#v", notified)
	}
}

func TestUpdateTaskOperationConfigDoesNotFailWhenNotificationFails(t *testing.T) {
	db := newTaskCenterTestDB(t)
	insertTaskDefinition(t, db, "task:config", model.TaskRecordSuccessAll)
	restoreNotify := setTaskDefinitionChangeNotifierForTest(func(taskCode string) error {
		if taskCode != "task:config" {
			t.Fatalf("unexpected task code: %s", taskCode)
		}
		return stderrors.New("redis down")
	})
	defer restoreNotify()
	restoreDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:        taskCode,
			Kind:        model.TaskKindCron,
			CronSpec:    "0 0 3 * * *",
			Status:      model.TaskStatusEnabled,
			AllowManual: model.TaskManualNotAllowed,
			AllowRetry:  model.TaskRetryAllowed,
		}, nil
	})
	defer restoreDefinition()

	_, err := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db}).UpdateTaskOperationConfig(context.Background(), &form.TaskOperationConfigForm{
		TaskCode:    "task:config",
		Status:      model.TaskStatusDisabled,
		AllowManual: model.TaskManualAllowed,
		AllowRetry:  model.TaskRetryNotAllowed,
		CronSpec:    "0 0 4 * * *",
	})
	if err != nil {
		t.Fatalf("expected notification failure not to block update, got %v", err)
	}
}

func TestUpdateTaskOperationConfigRejectsInvalidCronSpec(t *testing.T) {
	db := newTaskCenterTestDB(t)
	insertTaskDefinition(t, db, "task:config", model.TaskRecordSuccessAll)
	restoreDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:     taskCode,
			Kind:     model.TaskKindCron,
			CronSpec: "0 0 3 * * *",
			Status:   model.TaskStatusEnabled,
		}, nil
	})
	defer restoreDefinition()

	_, err := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db}).UpdateTaskOperationConfig(context.Background(), &form.TaskOperationConfigForm{
		TaskCode: "task:config",
		Status:   model.TaskStatusEnabled,
		CronSpec: "bad spec",
	})
	if err == nil {
		t.Fatal("expected invalid cron spec to be rejected")
	}
	var be *e.BusinessError
	if !stderrors.As(err, &be) {
		t.Fatalf("expected business error, got %T", err)
	}
	if be.GetCode() != e.InvalidParameter {
		t.Fatalf("expected code %d, got %d", e.InvalidParameter, be.GetCode())
	}
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
	restoreRetrySummary := setTaskRetryChainSummaryLoaderForTest(func(ctx context.Context, db *gorm.DB, rootRunID uint) (retryChainSummary, error) {
		_ = ctx
		_ = db
		_ = rootRunID
		return retryChainSummary{}, nil
	})
	defer restoreRetrySummary()

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

func TestRetryTaskRejectsSuccessfulRun(t *testing.T) {
	restoreRun := setTaskRunLoaderForTest(func(runID uint) (*model.TaskRun, error) {
		return &model.TaskRun{
			BaseModel: model.BaseModel{ID: runID},
			TaskCode:  "demo:send",
			Kind:      model.TaskKindAsync,
			Status:    model.TaskRunStatusSuccess,
		}, nil
	})
	defer restoreRun()

	svc := NewTaskCenterService()
	_, err := svc.RetryTask(context.Background(), 101, 1, "tester")
	if err == nil {
		t.Fatal("expected error when retrying successful run")
	}
	var be *e.BusinessError
	if !stderrors.As(err, &be) {
		t.Fatalf("expected business error, got %T", err)
	}
	if be.GetCode() != e.InvalidParameter {
		t.Fatalf("expected code %d, got %d", e.InvalidParameter, be.GetCode())
	}
}

func TestRetryTaskRejectsResolvedOrExceededChain(t *testing.T) {
	restoreRun := setTaskRunLoaderForTest(func(runID uint) (*model.TaskRun, error) {
		return &model.TaskRun{
			BaseModel: model.BaseModel{ID: runID},
			TaskCode:  "demo:send",
			Kind:      model.TaskKindAsync,
			Status:    model.TaskRunStatusFailed,
			MaxRetry:  2,
		}, nil
	})
	defer restoreRun()

	restoreDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:       taskCode,
			Kind:       model.TaskKindAsync,
			Status:     model.TaskStatusEnabled,
			AllowRetry: model.TaskRetryAllowed,
		}, nil
	})
	defer restoreDefinition()

	restoreRetrySummary := setTaskRetryChainSummaryLoaderForTest(func(ctx context.Context, db *gorm.DB, rootRunID uint) (retryChainSummary, error) {
		_ = ctx
		_ = db
		_ = rootRunID
		return retryChainSummary{MaxSeq: 2}, nil
	})
	defer restoreRetrySummary()

	svc := NewTaskCenterService()
	_, err := svc.RetryTask(context.Background(), 101, 1, "tester")
	if err == nil {
		t.Fatal("expected error when manual retry limit is reached")
	}
	var be *e.BusinessError
	if !stderrors.As(err, &be) {
		t.Fatalf("expected business error, got %T", err)
	}
	if be.GetCode() != e.InvalidParameter {
		t.Fatalf("expected code %d, got %d", e.InvalidParameter, be.GetCode())
	}
}

func TestRetryTaskRejectsSuccessfulRetryChain(t *testing.T) {
	restoreRun := setTaskRunLoaderForTest(func(runID uint) (*model.TaskRun, error) {
		return &model.TaskRun{
			BaseModel: model.BaseModel{ID: runID},
			TaskCode:  "demo:send",
			Kind:      model.TaskKindAsync,
			Status:    model.TaskRunStatusFailed,
			MaxRetry:  3,
		}, nil
	})
	defer restoreRun()

	restoreDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:       taskCode,
			Kind:       model.TaskKindAsync,
			Status:     model.TaskStatusEnabled,
			AllowRetry: model.TaskRetryAllowed,
		}, nil
	})
	defer restoreDefinition()

	restoreRetrySummary := setTaskRetryChainSummaryLoaderForTest(func(ctx context.Context, db *gorm.DB, rootRunID uint) (retryChainSummary, error) {
		_ = ctx
		_ = db
		_ = rootRunID
		return retryChainSummary{MaxSeq: 1, SuccessCount: 1}, nil
	})
	defer restoreRetrySummary()

	_, err := NewTaskCenterService().RetryTask(context.Background(), 101, 1, "tester")
	if err == nil {
		t.Fatal("expected error when retry chain has already succeeded")
	}
	var be *e.BusinessError
	if !stderrors.As(err, &be) {
		t.Fatalf("expected business error, got %T", err)
	}
	if be.GetCode() != e.InvalidParameter {
		t.Fatalf("expected code %d, got %d", e.InvalidParameter, be.GetCode())
	}
}

func TestRetryTaskCarriesRetryChain(t *testing.T) {
	restoreRun := setTaskRunLoaderForTest(func(runID uint) (*model.TaskRun, error) {
		return &model.TaskRun{
			BaseModel:      model.BaseModel{ID: runID},
			TaskCode:       "demo:send",
			Kind:           model.TaskKindAsync,
			Queue:          "default",
			Status:         model.TaskRunStatusFailed,
			MaxRetry:       3,
			RetryRootRunID: 88,
			RetrySeq:       1,
			Payload:        `{"name":"codex"}`,
		}, nil
	})
	defer restoreRun()

	restoreDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:       taskCode,
			Kind:       model.TaskKindAsync,
			Status:     model.TaskStatusEnabled,
			AllowRetry: model.TaskRetryAllowed,
		}, nil
	})
	defer restoreDefinition()

	restoreRetrySummary := setTaskRetryChainSummaryLoaderForTest(func(ctx context.Context, db *gorm.DB, rootRunID uint) (retryChainSummary, error) {
		_ = ctx
		_ = db
		if rootRunID != 88 {
			t.Fatalf("expected retry root 88, got %d", rootRunID)
		}
		return retryChainSummary{MaxSeq: 1}, nil
	})
	defer restoreRetrySummary()

	restorePublisher := queue.SetPublisherForTesting(&stubPublisher{
		info: queue.JobInfo{ID: "task-retry-1", Queue: "default", Type: "demo:send"},
	})
	defer restorePublisher()
	fakeRecorder := &fakeActionRecorder{}
	restoreRecorder := SetRecorderForTesting(fakeRecorder)
	defer restoreRecorder()
	exportSvc := &stubExportTaskService{}
	restoreExportSvc := setExportTaskServiceForTesting(exportSvc)
	defer restoreExportSvc()

	result, err := NewTaskCenterService().RetryTask(context.Background(), 101, 9, "tester")
	if err != nil {
		t.Fatalf("RetryTask returned error: %v", err)
	}
	if result["retry_from_run"] != uint(101) || result["retry_root_run"] != uint(88) || result["retry_seq"] != 2 {
		t.Fatalf("unexpected retry result: %#v", result)
	}
	if len(fakeRecorder.enqueueInputs) != 1 {
		t.Fatalf("expected enqueue once, got %d", len(fakeRecorder.enqueueInputs))
	}
	input := fakeRecorder.enqueueInputs[0]
	if input.RetryOfRunID != 101 || input.RetryRootRunID != 88 || input.RetrySeq != 2 {
		t.Fatalf("unexpected retry chain input: %#v", input)
	}
}

func TestRetryTaskDBPathRejectsSecondPendingRetry(t *testing.T) {
	db := newTaskCenterTestDB(t)
	insertTaskDefinition(t, db, "demo:send", model.TaskRecordSuccessAll)
	if err := db.Exec(`INSERT INTO task_runs (id, task_code, kind, source, source_id, queue, status, max_retry, payload) VALUES (101, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"demo:send",
		model.TaskKindAsync,
		model.TaskSourceQueue,
		"origin-task",
		"default",
		model.TaskRunStatusFailed,
		3,
		`{"name":"codex"}`,
	).Error; err != nil {
		t.Fatalf("insert task run failed: %v", err)
	}

	restoreRun := setTaskRunLoaderForTest(func(runID uint) (*model.TaskRun, error) {
		var run model.TaskRun
		if err := db.First(&run, runID).Error; err != nil {
			return nil, err
		}
		return &run, nil
	})
	defer restoreRun()
	restoreDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		var definition model.TaskDefinition
		if err := db.Where("code = ? AND deleted_at = 0", taskCode).First(&definition).Error; err != nil {
			return nil, err
		}
		return &definition, nil
	})
	defer restoreDefinition()

	restorePublisher := queue.SetPublisherForTesting(&stubPublisher{
		info: queue.JobInfo{ID: "task-retry-1", Queue: "default", Type: "demo:send"},
	})
	defer restorePublisher()
	exportSvc := &stubExportTaskService{}
	restoreExportSvc := setExportTaskServiceForTesting(exportSvc)
	defer restoreExportSvc()

	svc := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db})
	result, err := svc.RetryTask(context.Background(), 101, 9, "tester")
	if err != nil {
		t.Fatalf("first RetryTask returned error: %v", err)
	}
	if result["retry_seq"] != 1 {
		t.Fatalf("unexpected retry seq: %#v", result)
	}

	_, err = svc.RetryTask(context.Background(), 101, 9, "tester")
	if err == nil {
		t.Fatal("expected second retry to be rejected while first retry is pending")
	}
	var be *e.BusinessError
	if !stderrors.As(err, &be) {
		t.Fatalf("expected business error, got %T", err)
	}
	if be.GetCode() != e.InvalidParameter {
		t.Fatalf("expected code %d, got %d", e.InvalidParameter, be.GetCode())
	}

	var pendingCount int64
	if err := db.Model(&model.TaskRun{}).
		Where("retry_root_run_id = ? AND status = ?", 101, model.TaskRunStatusPending).
		Count(&pendingCount).Error; err != nil {
		t.Fatalf("count pending retry runs failed: %v", err)
	}
	if pendingCount != 1 {
		t.Fatalf("expected exactly one pending retry run, got %d", pendingCount)
	}
}

func TestRetryTaskReassignsExistingExportRecordAndInjectsNewTaskRunID(t *testing.T) {
	restoreRun := setTaskRunLoaderForTest(func(runID uint) (*model.TaskRun, error) {
		return &model.TaskRun{
			BaseModel: model.BaseModel{ID: runID},
			TaskCode:  "export:request-log",
			Kind:      model.TaskKindAsync,
			Queue:     "export",
			Status:    model.TaskRunStatusFailed,
			Payload:   `{"limit":50}`,
		}, nil
	})
	defer restoreRun()

	restoreDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:       taskCode,
			Kind:       model.TaskKindAsync,
			Status:     model.TaskStatusEnabled,
			AllowRetry: 1,
		}, nil
	})
	defer restoreDefinition()
	restoreRetrySummary := setTaskRetryChainSummaryLoaderForTest(func(ctx context.Context, db *gorm.DB, rootRunID uint) (retryChainSummary, error) {
		_ = ctx
		_ = db
		_ = rootRunID
		return retryChainSummary{}, nil
	})
	defer restoreRetrySummary()

	publisher := &stubPublisher{
		info: queue.JobInfo{ID: "task-export-1", Queue: "export", Type: "export:request-log"},
	}
	restorePublisher := queue.SetPublisherForTesting(publisher)
	defer restorePublisher()

	fakeRecorder := &fakeActionRecorder{}
	restoreRecorder := SetRecorderForTesting(fakeRecorder)
	defer restoreRecorder()

	exportSvc := &stubExportTaskService{hasRecordReturn: true, reassignReturn: true}
	restoreExportSvc := setExportTaskServiceForTesting(exportSvc)
	defer restoreExportSvc()

	svc := NewTaskCenterService()
	result, err := svc.RetryTask(context.Background(), 101, 9, "tester")
	if err != nil {
		t.Fatalf("RetryTask returned error: %v", err)
	}
	if result["run_id"] != uint(1) {
		t.Fatalf("unexpected retry run id: %#v", result["run_id"])
	}
	if publisher.job == nil {
		t.Fatal("expected job to be published")
	}
	if len(exportSvc.hasRecordInputs) != 1 || exportSvc.hasRecordInputs[0] != 101 {
		t.Fatalf("expected HasRecordForTaskRun called with 101, got %#v", exportSvc.hasRecordInputs)
	}
	if len(exportSvc.reassignInputs) != 1 {
		t.Fatalf("expected ReassignTaskRun called once, got %d", len(exportSvc.reassignInputs))
	}
	if exportSvc.reassignInputs[0].oldTaskRunID != 101 || exportSvc.reassignInputs[0].newTaskRunID != 1 || exportSvc.reassignInputs[0].queueName != "export" {
		t.Fatalf("unexpected reassign args: %#v", exportSvc.reassignInputs[0])
	}
	if len(exportSvc.createRecordInputs) != 0 {
		t.Fatalf("did not expect CreateRecordForTask for retry, got %d", len(exportSvc.createRecordInputs))
	}

	rawPayload, err := publisher.job.Payload()
	if err != nil {
		t.Fatalf("read published payload failed: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		t.Fatalf("decode published payload failed: %v", err)
	}
	if payload["task_run_id"] != float64(1) {
		t.Fatalf("expected published payload task_run_id=1, got %#v", payload["task_run_id"])
	}
	if payload["limit"] != float64(50) {
		t.Fatalf("unexpected original payload fields: %#v", payload)
	}
}

func TestRetryTaskRejectsExportAfterThreeManualRetries(t *testing.T) {
	db := newTaskCenterTestDB(t)
	if err := db.Create(&model.TaskRun{
		BaseModel: model.BaseModel{ID: 101},
		TaskCode:  "export:request-log",
		Kind:      model.TaskKindAsync,
		Queue:     "export",
		Status:    model.TaskRunStatusFailed,
		MaxRetry:  99,
	}).Error; err != nil {
		t.Fatalf("insert root task run failed: %v", err)
	}
	restoreRun := setTaskRunLoaderForTest(func(runID uint) (*model.TaskRun, error) {
		return &model.TaskRun{
			BaseModel: model.BaseModel{ID: runID},
			TaskCode:  "export:request-log",
			Kind:      model.TaskKindAsync,
			Queue:     "export",
			Status:    model.TaskRunStatusFailed,
			MaxRetry:  99,
		}, nil
	})
	defer restoreRun()

	restoreDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:       taskCode,
			Kind:       model.TaskKindAsync,
			Status:     model.TaskStatusEnabled,
			AllowRetry: 1,
		}, nil
	})
	defer restoreDefinition()
	restoreRetrySummary := setTaskRetryChainSummaryLoaderForTest(func(ctx context.Context, db *gorm.DB, rootRunID uint) (retryChainSummary, error) {
		_ = ctx
		_ = db
		_ = rootRunID
		return retryChainSummary{MaxSeq: 3}, nil
	})
	defer restoreRetrySummary()

	exportSvc := &stubExportTaskService{hasRecordReturn: true}
	restoreExportSvc := setExportTaskServiceForTesting(exportSvc)
	defer restoreExportSvc()

	_, err := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db}).RetryTask(context.Background(), 101, 9, "tester")
	if err == nil {
		t.Fatal("expected export retry to be rejected after three manual retries")
	}
	var be *e.BusinessError
	if !stderrors.As(err, &be) {
		t.Fatalf("expected business error, got %T", err)
	}
	if be.GetCode() != e.InvalidParameter {
		t.Fatalf("expected code %d, got %d", e.InvalidParameter, be.GetCode())
	}
	if !strings.Contains(be.GetMessage(), "3") {
		t.Fatalf("expected retry limit message to mention 3, got %s", be.GetMessage())
	}
}

func TestRetryTaskRejectsExportWhenRecordIsNotCurrent(t *testing.T) {
	restoreRun := setTaskRunLoaderForTest(func(runID uint) (*model.TaskRun, error) {
		return &model.TaskRun{
			BaseModel: model.BaseModel{ID: runID},
			TaskCode:  "export:request-log",
			Kind:      model.TaskKindAsync,
			Queue:     "export",
			Status:    model.TaskRunStatusFailed,
		}, nil
	})
	defer restoreRun()

	restoreDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:       taskCode,
			Kind:       model.TaskKindAsync,
			Status:     model.TaskStatusEnabled,
			AllowRetry: 1,
		}, nil
	})
	defer restoreDefinition()

	fakeRecorder := &fakeActionRecorder{}
	restoreRecorder := SetRecorderForTesting(fakeRecorder)
	defer restoreRecorder()
	exportSvc := &stubExportTaskService{hasRecordReturn: false}
	restoreExportSvc := setExportTaskServiceForTesting(exportSvc)
	defer restoreExportSvc()

	_, err := NewTaskCenterService().RetryTask(context.Background(), 101, 9, "tester")
	if err == nil {
		t.Fatal("expected missing export record to reject retry")
	}
	var be *e.BusinessError
	if !stderrors.As(err, &be) {
		t.Fatalf("expected business error, got %T", err)
	}
	if be.GetCode() != e.NotFound {
		t.Fatalf("expected code %d, got %d", e.NotFound, be.GetCode())
	}
	if len(fakeRecorder.enqueueInputs) != 0 {
		t.Fatalf("did not expect retry run to be enqueued when export record is missing: %#v", fakeRecorder.enqueueInputs)
	}
	if len(exportSvc.reassignInputs) != 0 {
		t.Fatalf("did not expect reassign when export record is missing: %#v", exportSvc.reassignInputs)
	}
}

func TestRetryTaskDoesNotInjectTaskRunIDForNonExportTasks(t *testing.T) {
	restoreRun := setTaskRunLoaderForTest(func(runID uint) (*model.TaskRun, error) {
		return &model.TaskRun{
			BaseModel: model.BaseModel{ID: runID},
			TaskCode:  "demo:send",
			Kind:      model.TaskKindAsync,
			Queue:     "default",
			Status:    model.TaskRunStatusFailed,
			Payload:   `{"limit":50}`,
		}, nil
	})
	defer restoreRun()

	restoreDefinition := setTaskDefinitionLoaderForTest(func(taskCode string) (*model.TaskDefinition, error) {
		return &model.TaskDefinition{
			Code:       taskCode,
			Kind:       model.TaskKindAsync,
			Status:     model.TaskStatusEnabled,
			AllowRetry: 1,
		}, nil
	})
	defer restoreDefinition()
	restoreRetrySummary := setTaskRetryChainSummaryLoaderForTest(func(ctx context.Context, db *gorm.DB, rootRunID uint) (retryChainSummary, error) {
		_ = ctx
		_ = db
		_ = rootRunID
		return retryChainSummary{}, nil
	})
	defer restoreRetrySummary()

	publisher := &stubPublisher{
		info: queue.JobInfo{ID: "task-demo-1", Queue: "default", Type: "demo:send"},
	}
	restorePublisher := queue.SetPublisherForTesting(publisher)
	defer restorePublisher()

	fakeRecorder := &fakeActionRecorder{}
	restoreRecorder := SetRecorderForTesting(fakeRecorder)
	defer restoreRecorder()

	exportSvc := &stubExportTaskService{createRecordMatched: false}
	restoreExportSvc := setExportTaskServiceForTesting(exportSvc)
	defer restoreExportSvc()

	svc := NewTaskCenterService()
	result, err := svc.RetryTask(context.Background(), 101, 9, "tester")
	if err != nil {
		t.Fatalf("RetryTask returned error: %v", err)
	}
	if result["run_id"] != uint(1) {
		t.Fatalf("unexpected retry run id: %#v", result["run_id"])
	}
	if len(exportSvc.createRecordInputs) != 0 {
		t.Fatalf("did not expect CreateRecordForTask for non-export retry, got %d", len(exportSvc.createRecordInputs))
	}
	if len(exportSvc.reassignInputs) != 0 {
		t.Fatalf("did not expect ReassignTaskRun for non-export retry, got %d", len(exportSvc.reassignInputs))
	}
	if publisher.job == nil {
		t.Fatal("expected job to be published")
	}
	rawPayload, err := publisher.job.Payload()
	if err != nil {
		t.Fatalf("read published payload failed: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		t.Fatalf("decode published payload failed: %v", err)
	}
	if _, ok := payload["task_run_id"]; ok {
		t.Fatalf("did not expect non-export retry payload to include task_run_id: %#v", payload)
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

func setTaskRetryChainSummaryLoaderForTest(loader func(ctx context.Context, db *gorm.DB, rootRunID uint) (retryChainSummary, error)) func() {
	previous := loadTaskRetryChainSummary
	loadTaskRetryChainSummary = loader
	return func() {
		loadTaskRetryChainSummary = previous
	}
}

func setCronExecutorForTest(executor func(ctx context.Context, handler string, payload map[string]any) error) func() {
	previous := executeCronHandler
	executeCronHandler = executor
	return func() {
		executeCronHandler = previous
	}
}

func setTaskDefinitionChangeNotifierForTest(notifier func(taskCode string) error) func() {
	previous := notifyTaskDefinitionChanged
	notifyTaskDefinitionChanged = notifier
	return func() {
		notifyTaskDefinitionChanged = previous
	}
}
