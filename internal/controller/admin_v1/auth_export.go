package admin_v1

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"uuid"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/taskcenter"
)

// asyncExportRequest 描述一次异步导出任务的差异化输入。
// taskCode 同时用作任务中心 TaskCode 与队列任务类型；payload 为该导出场景的业务过滤参数
// （无需预置 trigger_user_id，由编排逻辑统一注入）。
type asyncExportRequest struct {
	taskCode  string
	queueName string
	payload   map[string]any
	hasRows   func(context.Context) (bool, error)
}

// submitAsyncExport 统一编排异步导出："检查非空 -> 任务中心预建运行记录 -> 导出中心建记录 -> 投递队列"，
// 空集不会创建任务；后续步骤失败则记录失败状态并按错误类型写出 HTTP 响应。两种导出共用此流程，
// 避免两处控制器各自维护一份易漂移的编排副本。
func submitAsyncExport(api controller.Api, c *gin.Context, req asyncExportRequest) {
	if req.hasRows == nil {
		api.FailCode(c, e.ServerErr)
		return
	}
	hasRows, err := req.hasRows(c.Request.Context())
	if err != nil {
		api.Err(c, err)
		return
	}
	if !hasRows {
		api.FailCode(c, e.NoExportData)
		return
	}

	uid := api.GetCurrentUserID(c)
	account := ""
	if user := api.GetCurrentAdminUserSnapshot(c); user != nil {
		account = user.Username
	}

	if req.payload == nil {
		req.payload = map[string]any{}
	}
	req.payload["trigger_user_id"] = uid

	taskID := req.taskCode + ":" + uuid.New().String()
	rawPayload, _ := json.Marshal(req.payload)

	recorder := taskcenter.NewRunRecorder()
	// 先在任务中心预创建运行记录，供导出中心关联本次执行。
	run, err := recorder.Enqueue(c.Request.Context(), taskcenter.RunStart{
		TaskCode:       req.taskCode,
		Kind:           model.TaskKindAsync,
		Source:         model.TaskSourceManual,
		SourceID:       taskID,
		Queue:          req.queueName,
		Payload:        rawPayload,
		TriggerUserID:  uid,
		TriggerAccount: account,
	})
	if err != nil {
		api.Err(c, err)
		return
	}

	exportRecord, _, err := service.NewExportCenterService().CreateRecordForTask(c.Request.Context(), service.ExportTaskBootstrapInput{
		TaskCode:      req.taskCode,
		TaskRunID:     run.ID,
		TriggerUserID: uid,
		Queue:         req.queueName,
	})
	if err != nil {
		_ = recorder.Finish(c.Request.Context(), run, taskcenter.RunFinish{
			Status: model.TaskRunStatusFailed,
			Error:  err,
		})
		if errors.Is(err, service.ErrExportRecordTableMissing) {
			api.FailCode(c, e.ServiceDependencyNotReady)
			return
		}
		api.Err(c, err)
		return
	}

	req.payload["task_run_id"] = run.ID
	req.payload["public_id"] = exportRecord.PublicID

	// 再把真实导出任务投递到异步队列；worker 消费时会回写这条 run 记录的执行状态和结果。
	// 导出类任务使用保守的重试和超时限制，避免卡住的任务永久占用并发槽。
	_, publishErr := queue.PublishJSON(c.Request.Context(), req.taskCode, req.queueName, req.payload,
		queue.WithTaskID(taskID),
		queue.WithMaxRetry(2),
		queue.WithTimeout(5*time.Minute),
		queue.WithRetention(24*time.Hour),
	)
	if publishErr != nil {
		_ = recorder.Finish(c.Request.Context(), run, taskcenter.RunFinish{
			Status: model.TaskRunStatusFailed,
			Error:  publishErr,
		})
		_ = service.NewExportCenterService().MarkFailed(c.Request.Context(), run.ID, publishErr)
		if errors.Is(publishErr, queue.ErrPublisherUnavailable) {
			api.FailCode(c, e.ServiceDependencyNotReady)
			return
		}
		api.Err(c, publishErr)
		return
	}

	api.Success(c, map[string]any{
		"public_id": exportRecord.PublicID,
		"status":    exportRecord.Status,
	})
}
