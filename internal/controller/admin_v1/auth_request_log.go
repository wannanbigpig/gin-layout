package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/service/audit"
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

// Export 导出请求日志 CSV。
func (api RequestLogController) Export(c *gin.Context) {
	params := form.NewRequestLogExportQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	content, fileName, err := audit.NewRequestLogService().ExportCSV(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Data(200, "text/csv; charset=utf-8", content)
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
