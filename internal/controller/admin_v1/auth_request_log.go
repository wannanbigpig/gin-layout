package admin_v1

import (
	"encoding/json"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/audit"
	"github.com/wannanbigpig/gin-layout/internal/service/taskcenter"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// RequestLogController 请求日志控制器
type RequestLogController struct {
	controller.Api
}

// NewRequestLogController 创建请求日志控制器实例
func NewRequestLogController() *RequestLogController {
	return &RequestLogController{}
}

// List 分页查询请求日志列表
func (api RequestLogController) List(c *gin.Context) {
	params := form.NewRequestLogListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result := audit.NewRequestLogService().List(params)
	api.Success(c, result)
}

// Detail 获取请求日志详情
func (api RequestLogController) Detail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	detail, err := audit.NewRequestLogService().Detail(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, detail)
}

// Export 异步导出请求日志 CSV，提交到队列后返回任务信息。
func (api RequestLogController) Export(c *gin.Context) {
	params := form.NewRequestLogExportQuery()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	uid := api.GetCurrentUserID(c)
	account := ""
	if user := api.GetCurrentAdminUserSnapshot(c); user != nil {
		account = user.Username
	}

	taskID := jobs.TypeExportRequestLog + ":" + uuid.NewString()

	payload := map[string]any{
		"trigger_user_id":      uid,
		"operator_id":          params.OperatorID,
		"operator_account":     params.OperatorAccount,
		"operation_status":     params.OperationStatus,
		"is_high_risk":         params.IsHighRisk,
		"request_kind":         params.RequestKind,
		"execution_time_scope": params.ExecutionTimeScope,
		"method":               params.Method,
		"base_url":             params.BaseURL,
		"operation_name":       params.OperationName,
		"ip":                   params.IP,
		"start_time":           params.StartTime,
		"end_time":             params.EndTime,
		"limit":                params.Limit,
	}
	rawPayload, _ := json.Marshal(payload)

	recorder := taskcenter.NewRunRecorder()
	// 先在任务中心预创建一条运行记录，给导出中心和前端返回稳定的 run_id / 初始状态。
	run, err := recorder.Enqueue(c.Request.Context(), taskcenter.RunStart{
		TaskCode:       jobs.TypeExportRequestLog,
		Kind:           model.TaskKindAsync,
		Source:         model.TaskSourceManual,
		SourceID:       taskID,
		Queue:          jobs.ExportRequestLogQueue,
		Payload:        rawPayload,
		TriggerUserID:  uid,
		TriggerAccount: account,
	})
	if err != nil {
		api.Err(c, err)
		return
	}

	exportRecord, _, err := service.NewExportCenterService().CreateRecordForTask(c.Request.Context(), service.ExportTaskBootstrapInput{
		TaskCode:      jobs.TypeExportRequestLog,
		TaskRunID:     run.ID,
		TriggerUserID: uid,
		Queue:         jobs.ExportRequestLogQueue,
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

	payload["task_run_id"] = run.ID

	// 再把真实导出任务投递到异步队列；worker 消费时会回写这条 run 记录的执行状态和结果。
	jobInfo, publishErr := queue.PublishJSON(c.Request.Context(), jobs.TypeExportRequestLog, jobs.ExportRequestLogQueue, payload, queue.WithTaskID(taskID))
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
		"run_id":           run.ID,
		"export_record_id": exportRecord.ID,
		"export_record":    resources.NewExportRecordTransformer().ToStruct(exportRecord),
		"task_id":          jobInfo.ID,
		"queue":            jobInfo.Queue,
		"type":             jobInfo.Type,
	})
}

// MaskConfig 获取敏感字段脱敏配置。
func (api RequestLogController) MaskConfig(c *gin.Context) {
	result := audit.NewRequestLogService().GetMaskConfig()
	api.Success(c, result)
}

// UpdateMaskConfig 更新敏感字段脱敏配置（运行时生效）。
func (api RequestLogController) UpdateMaskConfig(c *gin.Context) {
	params := form.NewRequestLogMaskConfigForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	result, changeDiff, err := audit.NewRequestLogService().UpdateMaskConfigWithAuditDiff(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, result)
}
