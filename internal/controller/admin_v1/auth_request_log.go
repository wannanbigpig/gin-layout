package admin_v1

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service/audit"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// RequestLogController 请求日志控制器
type RequestLogController struct {
	controller.Api
}

type selfRequestLogService interface {
	SetCtx(*gin.Context)
	SelfList(*form.RequestLogList, uint) (*resources.Collection, error)
	SelfDetail(uint, uint) (any, error)
}

var newSelfRequestLogService = func() selfRequestLogService {
	return audit.NewRequestLogService()
}

// NewRequestLogController 创建请求日志控制器实例
func NewRequestLogController() *RequestLogController {
	return &RequestLogController{}
}

// SelfList 分页查询当前登录用户自己的请求日志。
func (api RequestLogController) SelfList(c *gin.Context) {
	params := form.NewRequestLogListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	svc := newSelfRequestLogService()
	svc.SetCtx(c)
	result, err := svc.SelfList(params, api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// SelfDetail 获取当前登录用户自己的请求日志详情。
func (api RequestLogController) SelfDetail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	svc := newSelfRequestLogService()
	svc.SetCtx(c)
	detail, err := svc.SelfDetail(query.ID, api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, detail)
}

// List 分页查询请求日志列表
func (api RequestLogController) List(c *gin.Context) {
	params := form.NewRequestLogListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	svc := audit.NewRequestLogService()
	svc.SetCtx(c)
	result, err := svc.List(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// Detail 获取请求日志详情
func (api RequestLogController) Detail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	svc := audit.NewRequestLogService()
	svc.SetCtx(c)
	detail, err := svc.Detail(query.ID)
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
	if err := audit.ValidateRequestLogTimeRange(params.StartTime, params.EndTime); err != nil {
		api.Err(c, err)
		return
	}
	payload := map[string]any{
		"request_id":           params.RequestID,
		"operator_id":          params.OperatorID,
		"operator_account":     params.OperatorAccount,
		"operation_status":     params.OperationStatus,
		"is_high_risk":         params.IsHighRisk,
		"has_change_diff":      params.HasChangeDiff,
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

	submitAsyncExport(api.Api, c, asyncExportRequest{
		taskCode:  jobs.TypeExportRequestLog,
		queueName: jobs.ExportRequestLogQueue,
		payload:   payload,
		hasRows: func(ctx context.Context) (bool, error) {
			return audit.HasRequestLogExportRows(ctx, params)
		},
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
