package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service/dept"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// DeptController 部门控制器
type DeptController struct {
	controller.Api
}

// NewDeptController 创建部门控制器实例
func NewDeptController() *DeptController {
	return &DeptController{}
}

// List 查询部门列表
func (api DeptController) List(c *gin.Context) {
	params := form.NewDeptListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result, err := dept.NewDeptService().List(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// Create 新增部门
func (api DeptController) Create(c *gin.Context) {
	handleAuditWrite(api.Api, c, form.NewCreateDeptForm, func(p *form.CreateDept) (string, error) {
		return dept.NewDeptService().CreateWithAuditDiff(p)
	})
}

// Update 更新部门
func (api DeptController) Update(c *gin.Context) {
	handleAuditWrite(api.Api, c, form.NewUpdateDeptForm, func(p *form.UpdateDept) (string, error) {
		return dept.NewDeptService().UpdateWithAuditDiff(p)
	})
}

// Delete 删除部门
func (api DeptController) Delete(c *gin.Context) {
	handleAuditWrite(api.Api, c, form.NewIdForm, func(p *form.ID) (string, error) {
		return dept.NewDeptService().DeleteWithAuditDiff(p.ID)
	})
}

// Detail 获取部门详情
func (api DeptController) Detail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	detail, err := dept.NewDeptService().Detail(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, detail)
}

// BindRole 绑定角色到部门
func (api DeptController) BindRole(c *gin.Context) {
	handleAuditWrite(api.Api, c, form.NewDeptBindRole, func(p *form.DeptBindRole) (string, error) {
		return dept.NewDeptService().BindRoleWithAuditDiff(p)
	})
}
