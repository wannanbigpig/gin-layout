package permission

import (
	"fmt"
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

// AdminUserService 授权服务
type AdminUserService struct {
	service.Base
}

func NewAdminUserService() *AdminUserService {
	return &AdminUserService{}
}

// GetUserInfo 获取用户信息
func (a *AdminUserService) GetUserInfo(id uint) (*resources.AdminUserResources, error) {
	// 查询用户是否存在
	adminUsersModel := model.NewAdminUsers()
	user := adminUsersModel.GetUserById(id)
	if user != nil {
		return resources.NewAdminUserTransformer().ToStruct(user), nil
	}
	return nil, e.NewBusinessError(e.FAILURE, "获取用户信息失败")
}

func (a *AdminUserService) List(params *form.AdminUserList) *resources.Collection {
	var condition strings.Builder
	var args []any
	if params.UserName != "" {
		condition.WriteString("username like ? AND ")
		args = append(args, "%"+params.UserName+"%")
	}
	if params.ID != 0 {
		condition.WriteString("id like ? AND ")
		args = append(args, params.ID)
	}
	if params.NickName != "" {
		condition.WriteString("nickname like ? AND ")
		args = append(args, "%"+params.NickName+"%")
	}
	if params.Email != "" {
		condition.WriteString("email like ? AND ")
		args = append(args, "%"+params.Email+"%")
	}
	if params.PhoneNumber != "" {
		condition.WriteString("full_phone_number like ? AND ")
		args = append(args, "%"+params.PhoneNumber+"%")
	}

	if params.Status != nil {
		condition.WriteString("status = ? AND ")
		args = append(args, params.Status)
	}

	conditionStr := condition.String()
	if conditionStr != "" {
		conditionStr = strings.TrimSuffix(condition.String(), "AND ")
	}

	adminUserModel := model.NewAdminUsers()
	ListOptionalParams := model.ListOptionalParams{
		OrderBy: "is_super_admin desc, created_at desc, id desc",
	}
	total, collection := model.ListPage[model.AdminUser](adminUserModel, params.Page, params.PerPage, conditionStr, args, ListOptionalParams)
	return resources.NewAdminUserTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

// Edit 编辑用户
func (a *AdminUserService) Edit(params *form.EditAdminUser) error {
	adminUserModel := model.NewAdminUsers()
	title := "新增"
	where := ""
	if params.Id > 0 {
		title = "更新"
		// 更新
		err := adminUserModel.GetById(adminUserModel, params.Id)
		if err != nil {
			return e.NewBusinessError(e.FAILURE, "用户不存在")
		}
		where = fmt.Sprintf(" AND id != %d", params.Id)
	} else {
		adminUserModel.Username = params.Username
	}
	// 校验唯一性
	err := a.validateUniqueFields(adminUserModel, params, where)
	if err != nil {
		return err
	}
	adminUserModel.PhoneNumber = params.PhoneNumber
	adminUserModel.CountryCode = utils2.If(params.CountryCode == "", global.ChinaCountryCode, params.CountryCode)
	adminUserModel.Email = params.Email
	if params.Password != "" {
		passwordHash, err := utils2.PasswordHash(params.Password)
		if err != nil {
			return e.NewBusinessError(e.FAILURE, "密码处理失败")
		}
		adminUserModel.Password = passwordHash
	}
	adminUserModel.Avatar = params.Avatar
	adminUserModel.Status = params.Status
	adminUserModel.Nickname = params.Nickname
	adminUserModel.FullPhoneNumber = adminUserModel.CountryCode + adminUserModel.PhoneNumber

	err = model.Save(adminUserModel)
	if err != nil {
		return e.NewBusinessError(e.FAILURE, title+"用户失败")
	}
	return nil
}

// validateUniqueFields 验证唯一性
func (a *AdminUserService) validateUniqueFields(adminUser *model.AdminUser, params *form.EditAdminUser, where string) error {
	// 验证用户名唯一性
	if params.Username != "" && adminUser.Exists(adminUser, "username = ?"+where, params.Username) {
		return e.NewBusinessError(1, "用户名已存在")
	}

	// 验证手机号
	fullPhoneNumber := utils2.If(params.CountryCode == "", global.ChinaCountryCode, params.CountryCode) + params.PhoneNumber
	if fullPhoneNumber != "" && adminUser.Exists(adminUser, "full_phone_number = ?"+where, fullPhoneNumber) {
		return e.NewBusinessError(1, "手机号已存在")
	}

	// 验证邮箱唯一性
	if adminUser.Email != "" && adminUser.Exists(adminUser, "email = ?"+where, params.Email) {
		return e.NewBusinessError(1, "邮箱已存在")
	}

	return nil
}

// Delete 删除用户
func (a *AdminUserService) Delete(id uint) error {
	if id == 1 {
		return e.NewBusinessError(1, "超级管理员不允许删除")
	}

	adminUserModel := model.NewAdminUsers()
	err := adminUserModel.Delete(adminUserModel, id)
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "删除用户失败")
	}
	return nil
}
