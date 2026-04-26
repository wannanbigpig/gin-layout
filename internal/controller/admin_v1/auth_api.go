package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/service/api_permission"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// ApiController API权限控制器
type ApiController struct {
	controller.Api
}

// NewApiController 创建API控制器实例
func NewApiController() *ApiController {
	return &ApiController{}
}

// Update 更新API权限
func (api ApiController) Update(c *gin.Context) {
	params := form.NewUpdateApiForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	changeDiff, err := api_permission.NewApiService().UpdateWithAuditDiff(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

// List 分页查询API权限列表
func (api ApiController) List(c *gin.Context) {
	params := form.NewListApiQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result := api_permission.NewApiService().ListPage(params)
	api.Success(c, result)
}
