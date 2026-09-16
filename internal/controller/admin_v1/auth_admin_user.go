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
	handleAuditWrite(api.Api, c, form.NewUpdateProfile, func(p *form.UpdateProfile) (string, error) {
		return admin.NewAdminUserService().UpdateProfileWithAuditDiff(uid, p)
	})
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
	handleAuditWrite(api.Api, c, form.NewCreateAdminUser, func(p *form.CreateAdminUser) (string, error) {
		service := admin.NewAdminUserService()
		service.SetAdminUserId(api.GetCurrentUserID(c))
		return service.CreateWithAuditDiff(p)
	})
}

// Update 更新管理员
func (api AdminUserController) Update(c *gin.Context) {
	handleAuditWrite(api.Api, c, form.NewUpdateAdminUser, func(p *form.UpdateAdminUser) (string, error) {
		service := admin.NewAdminUserService()
		service.SetCtx(c)
		service.SetAdminUserId(api.GetCurrentUserID(c))
		return service.UpdateWithAuditDiff(p)
	})
}

// List 分页查询管理员用户列表
func (api AdminUserController) List(c *gin.Context) {
	params := form.NewAdminUserListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	svc := admin.NewAdminUserService()
	svc.SetCtx(c)
	svc.SetAdminUserId(api.GetCurrentUserID(c))
	result, err := svc.List(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// Options 管理员选择器：按账号/昵称模糊搜索，供前端下拉远程搜索（如请求日志按操作人筛选）。
func (api AdminUserController) Options(c *gin.Context) {
	params := form.NewAdminUserOptionsQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result := admin.NewAdminUserService().Options(params.Keyword)
	api.Success(c, result)
}

// Delete 删除管理员
func (api AdminUserController) Delete(c *gin.Context) {
	handleAuditWrite(api.Api, c, form.NewIdForm, func(p *form.ID) (string, error) {
		service := admin.NewAdminUserService()
		service.SetCtx(c)
		service.SetAdminUserId(api.GetCurrentUserID(c))
		return service.DeleteWithAuditDiff(p.ID)
	})
}

// BindRole 管理员绑定角色
func (api AdminUserController) BindRole(c *gin.Context) {
	handleAuditWrite(api.Api, c, form.NewBindRole, func(p *form.BindRole) (string, error) {
		service := admin.NewAdminUserService()
		service.SetCtx(c)
		service.SetAdminUserId(api.GetCurrentUserID(c))
		return service.BindRoleWithAuditDiff(p)
	})
}

// Detail 获取管理员详情
func (api AdminUserController) Detail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	service := admin.NewAdminUserService()
	service.SetCtx(c)
	service.SetAdminUserId(api.GetCurrentUserID(c))
	detail, err := service.GetManagedUserInfo(query.ID)
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

	service := admin.NewAdminUserService()
	service.SetCtx(c)
	service.SetAdminUserId(api.GetCurrentUserID(c))
	userInfo, err := service.GetManagedUserInfo(query.ID)
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

	service := admin.NewAdminUserService()
	service.SetCtx(c)
	service.SetAdminUserId(api.GetCurrentUserID(c))
	userInfo, err := service.GetManagedUserInfo(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, map[string]string{
		"email": userInfo.Email,
	})
}

// UploadAvatar 上传头像
func (api AdminUserController) UploadAvatar(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		validator.ResponseError(c, err)
		return
	}
	uid := api.GetCurrentUserID(c)
	fileInfo, err := admin.NewAdminUserService().UploadAvatar(c.Request.Context(), uid, fileHeader)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, fileInfo)
}
