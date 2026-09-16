package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
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

// Create 新增菜单
func (api MenuController) Create(c *gin.Context) {
	locale := middleware.LocaleFromContext(c)
	handleAuditWrite(api.Api, c, form.NewCreateMenuForm, func(p *form.CreateMenu) (string, error) {
		return menu.NewMenuService().CreateWithAuditDiff(p, locale)
	})
}

// Update 更新菜单
func (api MenuController) Update(c *gin.Context) {
	locale := middleware.LocaleFromContext(c)
	handleAuditWrite(api.Api, c, form.NewUpdateMenuForm, func(p *form.UpdateMenu) (string, error) {
		return menu.NewMenuService().UpdateWithAuditDiff(p, locale)
	})
}

// UpdateAllMenuPermissions 批量更新菜单权限到casbin
func (api MenuController) UpdateAllMenuPermissions(c *gin.Context) {
	if err := menu.NewMenuService().UpdateAllMenuPermissions(); err != nil {
		api.Err(c, err)
		return
	}
	diff := auditdiff.Marshal(auditdiff.BuildFieldDiff(nil, map[string]any{
		"action": "sync_all_menu_permissions",
	}, []auditdiff.FieldRule{
		{Field: "action", Label: "操作"},
	}))
	middleware.SetAuditChangeDiffRaw(c, diff)
	api.Success(c, nil)
}

// Detail 获取菜单详情
func (api MenuController) Detail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	detail, err := menu.NewMenuService().Detail(query.ID, middleware.LocaleFromContext(c))
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
	result, err := menu.NewMenuService().List(params, middleware.LocaleFromContext(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// Delete 删除菜单
func (api MenuController) Delete(c *gin.Context) {
	handleAuditWrite(api.Api, c, form.NewIdForm, func(p *form.ID) (string, error) {
		return menu.NewMenuService().DeleteWithAuditDiff(p.ID)
	})
}
