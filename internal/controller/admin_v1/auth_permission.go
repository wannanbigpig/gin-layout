package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service/admin_auth"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type PermissionController struct {
	controller.Api
}

func NewPermissionController() *PermissionController {
	return &PermissionController{}
}

func (api PermissionController) Edit(c *gin.Context) {
	// 初始化参数结构体
	permissionForm := form.NewEditPermissionForm()
	// 绑定参数并使用验证器验证参数
	if err := validator.CheckPostParams(c, &permissionForm); err != nil {
		return
	}

	err := admin_auth.NewPermissionService().Edit(permissionForm)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}

func (api PermissionController) List(c *gin.Context) {
	// 初始化参数结构体
	permissionQuery := form.NewListPermissionQuery()
	// 绑定参数并使用验证器验证参数
	if err := validator.CheckQueryParams(c, &permissionQuery); err != nil {
		return
	}
	res := admin_auth.NewPermissionService().ListPage(permissionQuery)
	api.Success(c, res)
}
