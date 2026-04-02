package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service/menu"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// MenuController 菜单控制器
type MenuController struct {
	controller.Api
}

// NewMenuController 创建菜单控制器实例
func NewMenuController() *MenuController {
	return &MenuController{}
}

// Edit 编辑菜单
func (api MenuController) Edit(c *gin.Context) {
	params := form.NewUpdateMenuForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := menu.NewMenuService().Edit(params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// Create 新增菜单
func (api MenuController) Create(c *gin.Context) {
	params := form.NewCreateMenuForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := menu.NewMenuService().Create(params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// Update 更新菜单
func (api MenuController) Update(c *gin.Context) {
	params := form.NewUpdateMenuForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := menu.NewMenuService().Update(params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// UpdateAllMenuPermissions 批量更新菜单权限到casbin
func (api MenuController) UpdateAllMenuPermissions(c *gin.Context) {
	if err := menu.NewMenuService().UpdateAllMenuPermissions(); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// Detail 获取菜单详情
func (api MenuController) Detail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	detail, err := menu.NewMenuService().Detail(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, detail)
}

// List 查询菜单列表
func (api MenuController) List(c *gin.Context) {
	params := form.NewMenuListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	result := menu.NewMenuService().List(params)
	api.Success(c, result)
}

// Delete 删除菜单
func (api MenuController) Delete(c *gin.Context) {
	params := form.NewIdForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := menu.NewMenuService().Delete(params.ID); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}
