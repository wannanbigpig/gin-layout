package taskcenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"uuid"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/data"
	taskcron "github.com/wannanbigpig/gin-layout/internal/cron"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/cronexpr"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	baseservice "github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"go.uber.org/zap"
)

var loadTaskRunByID = func(runID uint) (*model.TaskRun, error) {
	runModel := model.NewTaskRun()
	if err := runModel.GetById(runID); err != nil {
		return nil, err
	}
	return runModel, nil
}

var loadTaskDefinitionByCode = func(taskCode string) (*model.TaskDefinition, error) {
	definition := model.NewTaskDefinition()
	if err := definition.GetDetail("code = ? AND deleted_at = 0", taskCode); err != nil {
		return nil, err
	}
	return definition, nil
}

var executeCronHandler = func(ctx context.Context, handler string, payload map[string]any) error {
	return taskcron.ExecuteHandler(ctx, handler, payload)
}

var notifyTaskDefinitionChanged = NotifyTaskDefinitionChanged

const taskLockReleaseTimeout = 3 * time.Second

type taskLock interface {
	Release(context.Context) error
}

// acquireTaskLock 获取高风险任务分布式锁。Redis 不可用时 fail-closed（拒绝执行）。
var acquireTaskLock = func(ctx context.Context, taskCode string) (taskLock, error) {
	client := data.RedisClient()
	if client == nil {
		return nil, e.NewBusinessError(e.ServiceDependencyNotReady, "Redis 不可用，无法获取高危任务锁")
	}
	lock := NewTaskLock(client, taskCode)
	if err := lock.Acquire(ctx); err != nil {
		if errors.Is(err, ErrTaskLockHeld) {
			return nil, e.NewBusinessError(e.InvalidParameter, "该高危任务正在其他实例执行中，请稍后重试")
		}
		return nil, e.NewBusinessError(e.ServiceDependencyNotReady, "Redis 不可用，无法获取高危任务锁")
	}
	return lock, nil
}

func releaseTaskLock(ctx context.Context, lock taskLock, taskCode string) {
	if ctx == nil {
		ctx = context.Background()
	}
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), taskLockReleaseTimeout)
	defer cancel()
	if err := lock.Release(releaseCtx); err != nil {
		log.Logger.Error("高危任务释放分布式锁失败",
			zap.String("task_code", taskCode),
			zap.Error(err))
	}
}

type exportTaskService interface {
	CreateRecordForTask(ctx context.Context, input baseservice.ExportTaskBootstrapInput) (*model.SysExportRecord, bool, error)
	HasRecordForTaskRun(ctx context.Context, taskRunID uint) (bool, error)
	ReassignTaskRun(ctx context.Context, oldTaskRunID, newTaskRunID uint, queueName string) (bool, error)
	MarkFailed(ctx context.Context, taskRunID uint, taskErr error) error
	MarkCanceledIfExists(ctx context.Context, taskRunID uint, cancelReason string) error
}

var newExportTaskService = func() exportTaskService {
	return baseservice.NewExportCenterService()
}

// TriggerTask 手动触发任务（支持 async 与 cron）。
func (s *TaskCenterService) TriggerTask(ctx context.Context, params *form.TaskTriggerForm, triggerUserID uint, triggerAccount string) (map[string]any, error) {
	taskCode := strings.TrimSpace(params.TaskCode)
	if taskCode == "" {
		return nil, e.NewBusinessError(e.InvalidParameter, "task_code 不能为空")
	}

	definition, err := loadTaskDefinitionByCode(taskCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.NotFound)
		}
		return nil, e.NewBusinessError(e.ServerErr, "读取任务定义失败")
	}
	if definition.Status != 1 {
		return nil, e.NewBusinessError(e.InvalidParameter, "任务已停用，无法触发")
	}
	if definition.AllowManual != 1 {
		return nil, e.NewBusinessError(e.InvalidParameter, "任务不允许手动触发")
	}
	if definition.IsHighRisk == model.TaskHighRisk && strings.TrimSpace(params.Confirm) == "" {
		return nil, e.NewBusinessError(e.InvalidParameter, "高危任务触发需要确认")
	}

	switch definition.Kind {
	case model.TaskKindAsync:
		return s.triggerAsyncTask(ctx, definition, params, triggerUserID, triggerAccount)
	case model.TaskKindCron:
		return s.triggerCronTask(ctx, definition, params, triggerUserID, triggerAccount)
	default:
		return nil, e.NewBusinessError(e.InvalidParameter, "当前任务类型不支持手动触发")
	}
}

// UpdateTaskRecordPolicy 更新任务成功明细记录策略。
func (s *TaskCenterService) UpdateTaskRecordPolicy(ctx context.Context, params *form.TaskRecordPolicyForm) (any, error) {
	taskCode := strings.TrimSpace(params.TaskCode)
	if taskCode == "" {
		return nil, e.NewBusinessError(e.InvalidParameter, "task_code 不能为空")
	}
	if params.RecordDetailOnFailure == 0 {
		return nil, e.NewBusinessError(e.InvalidParameter, "失败类结果必须记录明细")
	}

	definition, err := loadTaskDefinitionByCode(taskCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.NotFound)
		}
		return nil, e.NewBusinessError(e.ServerErr, "读取任务定义失败")
	}

	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	updates := map[string]any{
		"record_success_mode":             strings.TrimSpace(params.RecordSuccessMode),
		"record_success_rate":             params.RecordSuccessRate,
		"record_success_interval_seconds": params.RecordSuccessIntervalSeconds,
		"record_detail_on_manual":         params.RecordDetailOnManual,
		"record_detail_on_failure":        params.RecordDetailOnFailure,
	}
	if err := db.WithContext(ctx).Model(&model.TaskDefinition{}).
		Where("code = ? AND deleted_at = 0", taskCode).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	definition.RecordSuccessMode = strings.TrimSpace(params.RecordSuccessMode)
	definition.RecordSuccessRate = params.RecordSuccessRate
	definition.RecordSuccessIntervalSeconds = params.RecordSuccessIntervalSeconds
	definition.RecordDetailOnManual = params.RecordDetailOnManual
	definition.RecordDetailOnFailure = params.RecordDetailOnFailure
	return resources.NewTaskDefinitionTransformer().ToStruct(definition), nil
}

// UpdateTaskOperationConfig 更新任务启停、手动触发/重试开关和 cron 调度表达式。
func (s *TaskCenterService) UpdateTaskOperationConfig(ctx context.Context, params *form.TaskOperationConfigForm) (any, error) {
	taskCode := strings.TrimSpace(params.TaskCode)
	if taskCode == "" {
		return nil, e.NewBusinessError(e.InvalidParameter, "task_code 不能为空")
	}

	definition, err := loadTaskDefinitionByCode(taskCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.NotFound)
		}
		return nil, e.NewBusinessError(e.ServerErr, "读取任务定义失败")
	}

	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	updates := map[string]any{
		"status":       params.Status,
		"allow_manual": params.AllowManual,
		"allow_retry":  params.AllowRetry,
	}
	cronSpec := strings.TrimSpace(params.CronSpec)
	if definition.Kind == model.TaskKindCron {
		if cronSpec == "" {
			cronSpec = strings.TrimSpace(definition.CronSpec)
		}
		if cronSpec == "" {
			return nil, e.NewBusinessError(e.InvalidParameter, "cron_spec 不能为空")
		}
		if err := cronexpr.Validate(cronSpec); err != nil {
			return nil, e.NewBusinessError(e.InvalidParameter, "cron_spec 不是合法 Cron 表达式")
		}
		updates["cron_spec"] = cronSpec
	}
	if err := db.WithContext(ctx).Model(&model.TaskDefinition{}).
		Where("code = ? AND deleted_at = 0", taskCode).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := notifyTaskDefinitionChanged(taskCode); err != nil {
		log.Logger.Warn("发布任务定义变更通知失败",
			zap.String("task_code", taskCode),
			zap.Error(err))
	}
	definition.Status = params.Status
	definition.AllowManual = params.AllowManual
	definition.AllowRetry = params.AllowRetry
	if definition.Kind == model.TaskKindCron {
		definition.CronSpec = cronSpec
	}
	return resources.NewTaskDefinitionTransformer().ToStruct(definition), nil
}

func (s *TaskCenterService) triggerAsyncTask(ctx context.Context, definition *model.TaskDefinition, params *form.TaskTriggerForm, triggerUserID uint, triggerAccount string) (map[string]any, error) {
	queueName := strings.TrimSpace(params.Queue)
	if queueName == "" {
		queueName = strings.TrimSpace(definition.Queue)
	}
	if queueName == "" {
		queueName = queue.DefaultQueue
	}
	taskID := strings.TrimSpace(params.TaskID)
	if taskID == "" {
		taskID = "manual:" + uuid.New().String()
	}

	payload := map[string]any{}
	if params.Payload != nil {
		payload = params.Payload
	}
	rawPayload, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return nil, e.NewBusinessError(e.InvalidParameter, "payload 不是合法 JSON 对象")
	}

	recorder := NewRunRecorder()
	run, err := recorder.Enqueue(ctx, RunStart{
		TaskCode:       definition.Code,
		Kind:           definition.Kind,
		Source:         model.TaskSourceManual,
		SourceID:       taskID,
		Queue:          queueName,
		Payload:        rawPayload,
		TriggerUserID:  triggerUserID,
		TriggerAccount: triggerAccount,
		TriggerConfirm: strings.TrimSpace(params.Confirm),
		TriggerReason:  strings.TrimSpace(params.Reason),
	})
	if err != nil {
		return nil, err
	}

	// 导出类任务使用保守的重试和超时限制，避免卡住的任务永久占用并发槽；
	// 其他任务由 Asynq 默认策略处理（25 次重试、无超时）。
	options := []queue.JobOption{queue.WithTaskID(taskID)}
	if isExportTaskCode(definition.Code) {
		options = append(options,
			queue.WithMaxRetry(2),
			queue.WithTimeout(5*time.Minute),
			queue.WithRetention(24*time.Hour),
		)
	}
	jobInfo, publishErr := queue.PublishJSON(ctx, definition.Code, queueName, payload, options...)
	if publishErr != nil {
		finishErr := recorder.Finish(ctx, run, RunFinish{
			Status: model.TaskRunStatusFailed,
			Error:  publishErr,
		})
		if finishErr != nil {
			log.Logger.Warn("手动触发任务失败后更新执行记录失败",
				zap.Uint("run_id", run.ID),
				zap.Error(finishErr))
		}
		if errors.Is(publishErr, queue.ErrPublisherUnavailable) {
			return nil, e.NewBusinessError(e.ServiceDependencyNotReady, "队列未启用或未就绪")
		}
		return nil, e.NewBusinessError(e.ServerErr, "任务触发失败")
	}

	return map[string]any{
		"run_id":  run.ID,
		"task_id": jobInfo.ID,
		"queue":   jobInfo.Queue,
		"type":    jobInfo.Type,
	}, nil
}

func (s *TaskCenterService) triggerCronTask(ctx context.Context, definition *model.TaskDefinition, params *form.TaskTriggerForm, triggerUserID uint, triggerAccount string) (map[string]any, error) {
	if strings.TrimSpace(definition.Handler) == "" {
		return nil, e.NewBusinessError(e.InvalidParameter, "cron 任务处理器未配置")
	}

	taskID := strings.TrimSpace(params.TaskID)
	if taskID == "" {
		taskID = "manual:" + uuid.New().String()
	}

	payload := map[string]any{}
	if params.Payload != nil {
		payload = params.Payload
	}
	rawPayload, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return nil, e.NewBusinessError(e.InvalidParameter, "payload 不是合法 JSON 对象")
	}

	// 高危 cron 任务需要获取分布式锁，防止多实例并发执行。
	if definition.IsHighRisk == model.TaskHighRisk {
		lock, lockErr := acquireTaskLock(ctx, definition.Code)
		if lockErr != nil {
			return nil, lockErr
		}
		defer releaseTaskLock(ctx, lock, definition.Code)
	}

	run, err := NewRunRecorder().Start(ctx, RunStart{
		TaskCode:       definition.Code,
		Kind:           model.TaskKindCron,
		Source:         model.TaskSourceManual,
		SourceID:       taskID,
		CronSpec:       definition.CronSpec,
		Payload:        rawPayload,
		TriggerUserID:  triggerUserID,
		TriggerAccount: triggerAccount,
		TriggerConfirm: strings.TrimSpace(params.Confirm),
		TriggerReason:  strings.TrimSpace(params.Reason),
	})
	if err != nil {
		return nil, err
	}

	execErr := executeCronHandler(ctx, definition.Handler, payload)
	if finishErr := NewRunRecorder().Finish(ctx, run, RunFinish{Error: execErr}); finishErr != nil {
		log.Logger.Warn("手动触发 cron 任务后更新执行记录失败",
			zap.Uint("run_id", run.ID),
			zap.String("task_code", definition.Code),
			zap.Error(finishErr))
		return nil, e.NewBusinessError(e.ServerErr, "任务触发后更新执行记录失败")
	}

	if execErr != nil {
		return nil, e.NewBusinessError(e.ServerErr, "任务触发失败")
	}

	return map[string]any{
		"run_id":  run.ID,
		"task_id": taskID,
		"queue":   "",
		"type":    definition.Code,
	}, nil
}

// RetryTask 重试失败类任务，重试时会创建一条新的执行记录并挂到同一重试链路下。
func (s *TaskCenterService) RetryTask(ctx context.Context, runID uint, triggerUserID uint, triggerAccount string) (map[string]any, error) {
	runModel, err := loadTaskRunByID(runID)
	if err != nil || runModel == nil || runModel.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	if !isTaskRunRetryableStatus(runModel.Status) {
		return nil, e.NewBusinessError(e.InvalidParameter, "仅失败、超时或中断任务允许重试")
	}
	if strings.TrimSpace(runModel.TaskCode) == "" {
		return nil, e.NewBusinessError(e.InvalidParameter, "任务编码为空，无法重试")
	}

	// 校验当前任务定义是否允许重试。
	definition, err := loadTaskDefinitionByCode(runModel.TaskCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.NotFound, "任务定义不存在")
		}
		return nil, e.NewBusinessError(e.ServerErr, "读取任务定义失败")
	}
	if definition.Status != 1 {
		return nil, e.NewBusinessError(e.InvalidParameter, "任务已停用，无法重试")
	}
	if definition.AllowRetry != 1 {
		return nil, e.NewBusinessError(e.InvalidParameter, "任务不允许重试")
	}
	if definition.Kind != runModel.Kind {
		return nil, e.NewBusinessError(e.InvalidParameter, "任务类型与当前定义不一致，无法重试")
	}
	rootRunID := retryRootRunID(runModel)

	payloadAny := map[string]any{}
	if strings.TrimSpace(runModel.Payload) != "" {
		if err := json.Unmarshal([]byte(runModel.Payload), &payloadAny); err != nil {
			return nil, e.NewBusinessError(e.InvalidParameter, "任务 payload 非法，无法重试")
		}
	}

	queueName := strings.TrimSpace(runModel.Queue)
	if queueName == "" {
		queueName = queue.DefaultQueue
	}
	rawPayload, _ := json.Marshal(payloadAny)

	exportService := newExportTaskService()
	shouldUpdateExportRecord := isExportTaskCode(runModel.TaskCode)
	if shouldUpdateExportRecord {
		exists, err := exportService.HasRecordForTaskRun(ctx, runModel.ID)
		if err != nil {
			if errors.Is(err, baseservice.ErrExportRecordTableMissing) {
				return nil, e.NewBusinessError(e.ServiceDependencyNotReady, "导出记录表未初始化")
			}
			return nil, err
		}
		if !exists {
			return nil, e.NewBusinessError(e.NotFound, "导出记录不存在，无法重试")
		}
	}

	retryRun, retrySeq, taskID, err := s.enqueueRetryRunWithChainGuard(ctx, runModel, definition, rootRunID, queueName, rawPayload, triggerUserID, triggerAccount)
	if err != nil {
		return nil, err
	}

	if shouldUpdateExportRecord {
		reassigned, err := exportService.ReassignTaskRun(ctx, runModel.ID, retryRun.ID, queueName)
		if err != nil {
			_ = NewRunRecorder().Finish(ctx, retryRun, RunFinish{
				Status: model.TaskRunStatusFailed,
				Error:  err,
			})
			if errors.Is(err, baseservice.ErrExportRecordTableMissing) {
				return nil, e.NewBusinessError(e.ServiceDependencyNotReady, "导出记录表未初始化")
			}
			return nil, err
		}
		if !reassigned {
			reassignErr := e.NewBusinessError(e.NotFound, "导出记录不存在，无法重试")
			_ = NewRunRecorder().Finish(ctx, retryRun, RunFinish{
				Status: model.TaskRunStatusFailed,
				Error:  reassignErr,
			})
			return nil, reassignErr
		}
		payloadAny["task_run_id"] = retryRun.ID
	}

	options := []queue.JobOption{queue.WithTaskID(taskID)}
	if runModel.MaxRetry > 0 {
		options = append(options, queue.WithMaxRetry(runModel.MaxRetry))
	}
	jobInfo, publishErr := queue.PublishJSON(ctx, runModel.TaskCode, queueName, payloadAny, options...)
	if publishErr != nil {
		finishErr := NewRunRecorder().Finish(ctx, retryRun, RunFinish{
			Status: model.TaskRunStatusFailed,
			Error:  publishErr,
		})
		if finishErr != nil {
			log.Logger.Warn("重试任务发布失败后更新执行记录失败",
				zap.Uint("run_id", retryRun.ID),
				zap.Error(finishErr))
		}
		if shouldUpdateExportRecord {
			_ = exportService.MarkFailed(ctx, retryRun.ID, publishErr)
		}
		if errors.Is(publishErr, queue.ErrPublisherUnavailable) {
			return nil, e.NewBusinessError(e.ServiceDependencyNotReady, "队列未启用或未就绪")
		}
		return nil, e.NewBusinessError(e.ServerErr, "任务重试失败")
	}

	return map[string]any{
		"run_id":         retryRun.ID,
		"task_id":        jobInfo.ID,
		"queue":          jobInfo.Queue,
		"type":           jobInfo.Type,
		"retry_from_run": runModel.ID,
		"retry_root_run": rootRunID,
		"retry_seq":      retrySeq,
	}, nil
}

func (s *TaskCenterService) enqueueRetryRunWithChainGuard(ctx context.Context, runModel *model.TaskRun, definition *model.TaskDefinition, rootRunID uint, queueName string, rawPayload []byte, triggerUserID uint, triggerAccount string) (*model.TaskRun, int, string, error) {
	db, dbErr := s.dbOrDefault()
	if dbErr != nil {
		return enqueueRetryRunAfterChainCheck(ctx, nil, NewRunRecorder(), runModel, definition, rootRunID, queueName, rawPayload, triggerUserID, triggerAccount)
	}

	var retryRun *model.TaskRun
	var retrySeq int
	var taskID string
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked model.TaskRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("id = ?", rootRunID).
			First(&locked).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return e.NewBusinessError(e.NotFound, "重试根记录不存在")
			}
			log.Logger.Warn("锁定任务重试链路失败",
				zap.Uint("run_id", runModel.ID),
				zap.Uint("retry_root_run_id", rootRunID),
				zap.Error(err))
			return e.NewBusinessError(e.ServerErr, "锁定任务重试链路失败")
		}

		var err error
		retryRun, retrySeq, taskID, err = enqueueRetryRunAfterChainCheck(ctx, tx, NewRecorderWithDB(tx), runModel, definition, rootRunID, queueName, rawPayload, triggerUserID, triggerAccount)
		return err
	})
	if err != nil {
		return nil, 0, "", err
	}
	return retryRun, retrySeq, taskID, nil
}

func enqueueRetryRunAfterChainCheck(ctx context.Context, db *gorm.DB, recorder Recorder, runModel *model.TaskRun, definition *model.TaskDefinition, rootRunID uint, queueName string, rawPayload []byte, triggerUserID uint, triggerAccount string) (*model.TaskRun, int, string, error) {
	summary, err := loadTaskRetryChainSummary(ctx, db, rootRunID)
	if err != nil {
		log.Logger.Warn("读取任务重试链路失败",
			zap.Uint("run_id", runModel.ID),
			zap.Uint("retry_root_run_id", rootRunID),
			zap.Error(err))
		return nil, 0, "", e.NewBusinessError(e.ServerErr, "读取任务重试链路失败")
	}
	if canRetry, reason := evaluateTaskRunRetryState(runModel, definition, summary); !canRetry {
		return nil, 0, "", e.NewBusinessError(e.InvalidParameter, reason)
	}
	retrySeq := summary.MaxSeq + 1
	taskID := fmt.Sprintf("retry:%d:%d:%s", rootRunID, retrySeq, uuid.New().String())
	retryRun, err := recorder.Enqueue(ctx, RunStart{
		TaskCode:       runModel.TaskCode,
		Kind:           runModel.Kind,
		Source:         model.TaskSourceManual,
		SourceID:       taskID,
		Queue:          queueName,
		Attempt:        1,
		MaxRetry:       runModel.MaxRetry,
		RetryOfRunID:   runModel.ID,
		RetryRootRunID: rootRunID,
		RetrySeq:       retrySeq,
		Payload:        rawPayload,
		TriggerUserID:  triggerUserID,
		TriggerAccount: triggerAccount,
	})
	if err != nil {
		return nil, 0, "", err
	}
	return retryRun, retrySeq, taskID, nil
}

// CancelTask 取消待执行或执行中的异步任务。
func (s *TaskCenterService) CancelTask(ctx context.Context, runID uint, triggerUserID uint, triggerAccount string, cancelReason string) (map[string]any, error) {
	runModel, err := loadTaskRunByID(runID)
	if err != nil || runModel == nil || runModel.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}

	if runModel.Kind != model.TaskKindAsync {
		return nil, e.NewBusinessError(e.InvalidParameter, "仅异步任务支持取消")
	}
	if strings.TrimSpace(runModel.SourceID) == "" {
		return nil, e.NewBusinessError(e.InvalidParameter, "任务缺少 source_id，无法取消")
	}
	switch runModel.Status {
	case model.TaskRunStatusSuccess, model.TaskRunStatusFailed, model.TaskRunStatusCanceled:
		return nil, e.NewBusinessError(e.InvalidParameter, "已结束任务不支持取消")
	case model.TaskRunStatusPending, model.TaskRunStatusRetrying, model.TaskRunStatusRunning:
		// 允许取消
	default:
		return nil, e.NewBusinessError(e.InvalidParameter, "当前任务状态不支持取消")
	}

	queueName := strings.TrimSpace(runModel.Queue)
	if queueName == "" {
		queueName = queue.DefaultQueue
	}

	var cancelErr error
	if runModel.Status == model.TaskRunStatusRunning {
		cancelErr = queue.CancelProcessing(ctx, runModel.SourceID)
	} else {
		cancelErr = queue.DeleteTask(ctx, queueName, runModel.SourceID)
	}
	if cancelErr != nil {
		if errors.Is(cancelErr, queue.ErrInspectorUnavailable) {
			return nil, e.NewBusinessError(e.ServiceDependencyNotReady, "队列未启用或未就绪")
		}
		if errors.Is(cancelErr, queue.ErrTaskNotFound) || errors.Is(cancelErr, queue.ErrQueueNotFound) {
			return nil, e.NewBusinessError(e.NotFound, "队列任务不存在或已结束")
		}
		return nil, e.NewBusinessError(e.ServerErr, "任务取消失败")
	}

	runModel.Status = model.TaskRunStatusCanceled
	if err := NewRunRecorder().Finish(ctx, runModel, RunFinish{
		Status:            model.TaskRunStatusCanceled,
		CanceledBy:        triggerUserID,
		CanceledByAccount: triggerAccount,
		CancelReason:      strings.TrimSpace(cancelReason),
	}); err != nil {
		if errors.Is(err, ErrTaskRunStatusChanged) {
			return nil, e.NewBusinessError(e.InvalidParameter, "任务已完成或状态已变化，无法取消")
		}
		return nil, err
	}
	if err := newExportTaskService().MarkCanceledIfExists(ctx, runModel.ID, cancelReason); err != nil {
		log.Logger.Warn("取消任务后同步导出记录失败",
			zap.Uint("run_id", runModel.ID),
			zap.Error(err))
	}

	result := map[string]any{
		"run_id":              runModel.ID,
		"task_id":             runModel.SourceID,
		"status":              model.TaskRunStatusCanceled,
		"canceled_by":         triggerUserID,
		"canceled_by_account": triggerAccount,
	}
	if strings.TrimSpace(cancelReason) != "" {
		result["cancel_reason"] = strings.TrimSpace(cancelReason)
	}
	return result, nil
}
