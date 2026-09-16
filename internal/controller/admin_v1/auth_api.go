package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
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
	handleAuditWrite(api.Api, c, form.NewUpdateApiForm, func(p *form.UpdatePermission) (string, error) {
		return api_permission.NewApiService().UpdateWithAuditDiff(p)
	})
}

// List 分页查询API权限列表
func (api ApiController) List(c *gin.Context) {
	params := form.NewListApiQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result, err := api_permission.NewApiService().ListPage(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// Options 接口权限选择器：按名称/路由模糊搜索，供菜单编辑时下拉选择器使用。
func (api ApiController) Options(c *gin.Context) {
	params := form.NewPermissionOptionsQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result := api_permission.NewApiService().Options(params.Keyword, params.IsAuth)
	api.Success(c, result)
}
