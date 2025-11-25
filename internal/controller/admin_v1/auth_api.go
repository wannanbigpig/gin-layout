package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service/permission"
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

// Edit 编辑API权限
func (api ApiController) Edit(c *gin.Context) {
	params := form.NewEditApiForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := permission.NewApiService().Edit(params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// Create 新增API权限
func (api ApiController) Create(c *gin.Context) {
	params := form.NewEditApiForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	// 确保 ID 为空，表示新增
	params.Id = 0

	if err := permission.NewApiService().Edit(params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// Update 更新API权限
func (api ApiController) Update(c *gin.Context) {
	params := form.NewEditApiForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	// 确保 ID 不为空，表示更新
	if params.Id == 0 {
		api.Err(c, e.NewBusinessError(1, "更新API权限时ID不能为空"))
		return
	}

	if err := permission.NewApiService().Edit(params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// List 分页查询API权限列表
func (api ApiController) List(c *gin.Context) {
	params := form.NewListApiQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result := permission.NewApiService().ListPage(params)
	api.Success(c, result)
}
