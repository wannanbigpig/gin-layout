package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service/permission"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type MenuController struct {
	controller.Api
}

func NewMenuController() *MenuController {
	return &MenuController{}
}

func (api MenuController) Edit(c *gin.Context) {
	// 初始化参数结构体
	params := form.NewEditMenuForm()
	// 绑定参数并使用验证器验证参数
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	err := permission.NewMenuService().Edit(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}

func (api MenuController) Detail(c *gin.Context) {
	// 初始化参数结构体
	query := form.NewIdForm()
	// 绑定参数并使用验证器验证参数
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	detail, err := permission.NewMenuService().Detail(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, detail)
}

func (api MenuController) List(c *gin.Context) {
	// 初始化参数结构体
	query := form.NewMenuListQuery()

	// 绑定参数并使用验证器验证参数
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}
	res := permission.NewMenuService().List(query)
	api.Success(c, res)
}

func (api MenuController) Delete(c *gin.Context) {
	// 初始化参数结构体
	params := form.NewIdForm()

	// 绑定参数并使用验证器验证参数
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	err := permission.NewMenuService().Delete(params.ID)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}
