package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service/permission"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type AdminUserController struct {
	controller.Api
}

func NewAdminUserController() *AdminUserController {
	return &AdminUserController{}
}

func (api AdminUserController) GetUserInfo(c *gin.Context) {
	result, err := permission.NewAdminUserService().GetUserInfo(c.GetUint("uid"))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
	return
}

func (api AdminUserController) Edit(c *gin.Context) {
	// 初始化参数结构体
	Params := form.NewEditAdminUser()
	// // 绑定参数并使用验证器验证参数
	if err := validator.CheckPostParams(c, &Params); err != nil {
		return
	}
	err := permission.NewAdminUserService().Edit(Params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c)
	return
}

func (api AdminUserController) List(c *gin.Context) {
	// 初始化参数结构体
	params := form.NewAdminUserListQuery()
	// // 绑定参数并使用验证器验证参数
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	result := permission.NewAdminUserService().List(params)
	api.Success(c, result)
	return
}

func (api AdminUserController) Delete(c *gin.Context) {
	// 初始化参数结构体
	IDForm := form.NewIdForm()
	// // 绑定参数并使用验证器验证参数
	if err := validator.CheckPostParams(c, &IDForm); err != nil {
		return
	}
	err := permission.NewAdminUserService().Delete(IDForm.ID)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c)
	return
}
