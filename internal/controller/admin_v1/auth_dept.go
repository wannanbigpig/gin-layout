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

	result := dept.NewDeptService().List(params)
	api.Success(c, result)
}

// Create 新增部门
func (api DeptController) Create(c *gin.Context) {
	params := form.NewCreateDeptForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := dept.NewDeptService().Create(params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// Update 更新部门
func (api DeptController) Update(c *gin.Context) {
	params := form.NewUpdateDeptForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := dept.NewDeptService().Update(params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// Delete 删除部门
func (api DeptController) Delete(c *gin.Context) {
	params := form.NewIdForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := dept.NewDeptService().Delete(params.ID); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
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
	params := form.NewDeptBindRole()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := dept.NewDeptService().BindRole(params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}
