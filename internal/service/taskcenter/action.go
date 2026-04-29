package taskcenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	taskcron "github.com/wannanbigpig/gin-layout/internal/cron"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
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
		taskID = "manual:" + uuid.NewString()
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

	jobInfo, publishErr := queue.PublishJSON(ctx, definition.Code, queueName, payload, queue.WithTaskID(taskID))
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
		taskID = "manual:" + uuid.NewString()
	}

	payload := map[string]any{}
	if params.Payload != nil {
		payload = params.Payload
	}
	rawPayload, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return nil, e.NewBusinessError(e.InvalidParameter, "payload 不是合法 JSON 对象")
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

// RetryTask 重试失败任务，重试时会创建一条新的执行记录。
func (s *TaskCenterService) RetryTask(ctx context.Context, runID uint, triggerUserID uint, triggerAccount string) (map[string]any, error) {
	runModel, err := loadTaskRunByID(runID)
	if err != nil || runModel == nil || runModel.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	if runModel.Status != model.TaskRunStatusFailed {
		return nil, e.NewBusinessError(e.InvalidParameter, "仅失败任务允许重试")
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

	payloadAny := map[string]any{}
	if strings.TrimSpace(runModel.Payload) != "" {
		if err := json.Unmarshal([]byte(runModel.Payload), &payloadAny); err != nil {
			return nil, e.NewBusinessError(e.InvalidParameter, "任务 payload 非法，无法重试")
		}
	}

	taskID := fmt.Sprintf("retry:%d:%d:%s", runModel.ID, time.Now().UnixMilli(), uuid.NewString())
	queueName := strings.TrimSpace(runModel.Queue)
	if queueName == "" {
		queueName = queue.DefaultQueue
	}
	rawPayload, _ := json.Marshal(payloadAny)

	recorder := NewRunRecorder()
	retryRun, err := recorder.Enqueue(ctx, RunStart{
		TaskCode:       runModel.TaskCode,
		Kind:           runModel.Kind,
		Source:         model.TaskSourceManual,
		SourceID:       taskID,
		Queue:          queueName,
		Attempt:        1,
		MaxRetry:       runModel.MaxRetry,
		Payload:        rawPayload,
		TriggerUserID:  triggerUserID,
		TriggerAccount: triggerAccount,
	})
	if err != nil {
		return nil, err
	}

	options := []queue.JobOption{queue.WithTaskID(taskID)}
	if runModel.MaxRetry > 0 {
		options = append(options, queue.WithMaxRetry(runModel.MaxRetry))
	}
	jobInfo, publishErr := queue.PublishJSON(ctx, runModel.TaskCode, queueName, payloadAny, options...)
	if publishErr != nil {
		finishErr := recorder.Finish(ctx, retryRun, RunFinish{
			Status: model.TaskRunStatusFailed,
			Error:  publishErr,
		})
		if finishErr != nil {
			log.Logger.Warn("重试任务发布失败后更新执行记录失败",
				zap.Uint("run_id", retryRun.ID),
				zap.Error(finishErr))
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
	}, nil
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
		return nil, err
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
