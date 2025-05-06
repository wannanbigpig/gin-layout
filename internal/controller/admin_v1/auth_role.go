package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service/permission"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type RoleController struct {
	controller.Api
}

func NewRoleController() *RoleController {
	return &RoleController{}
}

func (api RoleController) List(c *gin.Context) {
	// 初始化参数结构体
	params := form.NewRoleListQuery()
	// // 绑定参数并使用验证器验证参数
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	result := permission.NewRoleService().List(params)
	api.Success(c, result)
	return
}

func (api RoleController) Edit(c *gin.Context) {

}
func (api RoleController) Delete(c *gin.Context) {

}
