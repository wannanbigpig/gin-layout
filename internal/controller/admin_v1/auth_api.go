package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service/permission"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type ApiController struct {
	controller.Api
}

func NewApiController() *ApiController {
	return &ApiController{}
}

func (api ApiController) Edit(c *gin.Context) {
	// 初始化参数结构体
	permissionForm := form.NewEditApiForm()
	// 绑定参数并使用验证器验证参数
	if err := validator.CheckPostParams(c, &permissionForm); err != nil {
		return
	}

	err := permission.NewApiService().Edit(permissionForm)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}

func (api ApiController) List(c *gin.Context) {
	// 初始化参数结构体
	permissionQuery := form.NewListApiQuery()
	// 绑定参数并使用验证器验证参数
	if err := validator.CheckQueryParams(c, &permissionQuery); err != nil {
		return
	}
	res := permission.NewApiService().ListPage(permissionQuery)
	api.Success(c, res)
}
