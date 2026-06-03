package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service/role"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// RoleController 角色控制器
type RoleController struct {
	controller.Api
}

// NewRoleController 创建角色控制器实例
func NewRoleController() *RoleController {
	return &RoleController{}
}

// List 分页查询角色列表
func (api RoleController) List(c *gin.Context) {
	params := form.NewRoleListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result := role.NewRoleService().List(params)
	api.Success(c, result)
}

// Create 新增角色
func (api RoleController) Create(c *gin.Context) {
	handleAuditWrite(api.Api, c, form.NewCreateRoleForm, func(p *form.CreateRole) (string, error) {
		return role.NewRoleService().CreateWithAuditDiff(p)
	})
}

// Update 更新角色
func (api RoleController) Update(c *gin.Context) {
	handleAuditWrite(api.Api, c, form.NewUpdateRoleForm, func(p *form.UpdateRole) (string, error) {
		return role.NewRoleService().UpdateWithAuditDiff(p)
	})
}

// Delete 删除角色
func (api RoleController) Delete(c *gin.Context) {
	handleAuditWrite(api.Api, c, form.NewIdForm, func(p *form.ID) (string, error) {
		return role.NewRoleService().DeleteWithAuditDiff(p.ID)
	})
}

// Detail 获取角色详情
func (api RoleController) Detail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	detail, err := role.NewRoleService().Detail(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, detail)
}
