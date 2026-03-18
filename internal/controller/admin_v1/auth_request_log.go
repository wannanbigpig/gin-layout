package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
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
