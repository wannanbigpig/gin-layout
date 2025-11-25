package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service/permission"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// AdminLoginLogController 登录日志控制器
type AdminLoginLogController struct {
	controller.Api
}

// NewAdminLoginLogController 创建登录日志控制器实例
func NewAdminLoginLogController() AdminLoginLogController {
	return AdminLoginLogController{}
}

// List 分页查询管理员登录日志列表
func (api AdminLoginLogController) List(c *gin.Context) {
	params := form.NewAdminLoginLogListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result := permission.NewAdminLoginLogService().List(params)
	api.Success(c, result)
}

// Detail 获取管理员登录日志详情
func (api AdminLoginLogController) Detail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	detail, err := permission.NewAdminLoginLogService().Detail(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, detail)
}
