package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/service/admin"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// AdminUserController 管理员用户控制器
type AdminUserController struct {
	controller.Api
}

// NewAdminUserController 创建管理员用户控制器实例
func NewAdminUserController() *AdminUserController {
	return &AdminUserController{}
}

// GetUserInfo 获取当前登录用户基本信息
func (api AdminUserController) GetUserInfo(c *gin.Context) {
	result, err := api.GetCurrentAdminUserDetail(c)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, result)
}

// UpdateProfile 更新个人资料（只能更新自己的手机号、邮箱、密码、昵称）
func (api AdminUserController) UpdateProfile(c *gin.Context) {
	uid := api.GetCurrentUserID(c)
	params := form.NewUpdateProfile()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	changeDiff, err := admin.NewAdminUserService().UpdateProfileWithAuditDiff(uid, params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

// GetUserMenuInfo 获取当前登录用户权限信息
func (api AdminUserController) GetUserMenuInfo(c *gin.Context) {
	uid := api.GetCurrentUserID(c)
	result, err := admin.NewAdminUserService().GetUserMenuInfo(uid, middleware.LocaleFromContext(c))
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, result)
}

// Create 新增管理员
func (api AdminUserController) Create(c *gin.Context) {
	params := form.NewCreateAdminUser()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	changeDiff, err := admin.NewAdminUserService().CreateWithAuditDiff(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

// Update 更新管理员
func (api AdminUserController) Update(c *gin.Context) {
	params := form.NewUpdateAdminUser()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	changeDiff, err := admin.NewAdminUserService().UpdateWithAuditDiff(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

// List 分页查询管理员用户列表
func (api AdminUserController) List(c *gin.Context) {
	params := form.NewAdminUserListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result := admin.NewAdminUserService().List(params)
	api.Success(c, result)
}

// Delete 删除管理员
func (api AdminUserController) Delete(c *gin.Context) {
	params := form.NewIdForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	changeDiff, err := admin.NewAdminUserService().DeleteWithAuditDiff(params.ID)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

// BindRole 管理员绑定角色
func (api AdminUserController) BindRole(c *gin.Context) {
	params := form.NewBindRole()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	changeDiff, err := admin.NewAdminUserService().BindRoleWithAuditDiff(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

// Detail 获取管理员详情
func (api AdminUserController) Detail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	detail, err := admin.NewAdminUserService().GetUserInfo(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, detail)
}

// GetFullPhone 获取管理员完整手机号
func (api AdminUserController) GetFullPhone(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	userInfo, err := admin.NewAdminUserService().GetUserInfo(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, map[string]string{
		"phone_number": userInfo.PhoneNumber,
	})
}

// GetFullEmail 获取管理员完整邮箱
func (api AdminUserController) GetFullEmail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	userInfo, err := admin.NewAdminUserService().GetUserInfo(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, map[string]string{
		"email": userInfo.Email,
	})
}
