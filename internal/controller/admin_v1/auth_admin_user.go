package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service/permission"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

const (
	contextKeyUID = "uid"
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
	uid := c.GetUint(contextKeyUID)
	result, err := permission.NewAdminUserService().GetUserInfo(uid)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, result)
}

// UpdateProfile 更新个人资料（只能更新自己的手机号、邮箱、密码、昵称）
func (api AdminUserController) UpdateProfile(c *gin.Context) {
	uid := c.GetUint(contextKeyUID)
	params := form.NewUpdateProfile()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := permission.NewAdminUserService().UpdateProfile(uid, params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// GetUserMenuInfo 获取当前登录用户权限信息
func (api AdminUserController) GetUserMenuInfo(c *gin.Context) {
	uid := c.GetUint(contextKeyUID)
	result, err := permission.NewAdminUserService().GetUserMenuInfo(uid)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, result)
}

// Edit 编辑管理员信息
func (api AdminUserController) Edit(c *gin.Context) {
	params := form.NewEditAdminUser()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := permission.NewAdminUserService().Edit(params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// Create 新增管理员
func (api AdminUserController) Create(c *gin.Context) {
	params := form.NewEditAdminUser()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	// 确保 ID 为空，表示新增
	params.Id = 0

	if err := permission.NewAdminUserService().Edit(params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// Update 更新管理员
func (api AdminUserController) Update(c *gin.Context) {
	params := form.NewEditAdminUser()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	// 确保 ID 不为空，表示更新
	if params.Id == 0 {
		api.Err(c, e.NewBusinessError(1, "更新管理员时ID不能为空"))
		return
	}

	if err := permission.NewAdminUserService().Edit(params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// List 分页查询管理员用户列表
func (api AdminUserController) List(c *gin.Context) {
	params := form.NewAdminUserListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result := permission.NewAdminUserService().List(params)
	api.Success(c, result)
}

// Delete 删除管理员
func (api AdminUserController) Delete(c *gin.Context) {
	params := form.NewIdForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := permission.NewAdminUserService().Delete(params.ID); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// BindRole 管理员绑定角色
func (api AdminUserController) BindRole(c *gin.Context) {
	params := form.NewBindRole()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := permission.NewAdminUserService().BindRole(params); err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, nil)
}

// Detail 获取管理员详情
func (api AdminUserController) Detail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	detail, err := permission.NewAdminUserService().GetUserInfo(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, detail)
}

// GetFullPhone 获取管理员完整手机号
func (api AdminUserController) GetFullPhone(c *gin.Context) {
	userInfo, err := api.getUserInfo(c)
	if err != nil {
		return
	}

	api.Success(c, map[string]string{
		"phone_number": userInfo.PhoneNumber,
	})
}

// GetFullEmail 获取管理员完整邮箱
func (api AdminUserController) GetFullEmail(c *gin.Context) {
	userInfo, err := api.getUserInfo(c)
	if err != nil {
		return
	}

	api.Success(c, map[string]string{
		"email": userInfo.Email,
	})
}

// getUserInfo 获取用户信息（内部辅助方法）
func (api AdminUserController) getUserInfo(c *gin.Context) (*resources.AdminUserResources, error) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return nil, err
	}

	result, err := permission.NewAdminUserService().GetUserInfo(query.ID)
	if err != nil {
		api.Err(c, err)
		return nil, err
	}

	return result, nil
}
