package admin

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

// Create 新增管理员用户。
func (s *AdminUserService) Create(params *form.CreateAdminUser) error {
	return s.saveAdminUserMutation(&adminUserEditParams{
		Username:    params.Username,
		Nickname:    params.Nickname,
		Password:    params.Password,
		PhoneNumber: params.PhoneNumber,
		CountryCode: params.CountryCode,
		Email:       params.Email,
		Status:      params.Status,
		Avatar:      params.Avatar,
		DeptIds:     params.DeptIds,
	})
}

// Update 更新管理员用户。
func (s *AdminUserService) Update(params *form.UpdateAdminUser) error {
	return s.saveAdminUserMutation(&adminUserEditParams{
		Id:          params.Id,
		Username:    params.Username,
		Nickname:    params.Nickname,
		Password:    params.Password,
		PhoneNumber: params.PhoneNumber,
		CountryCode: params.CountryCode,
		Email:       params.Email,
		Status:      params.Status,
		Avatar:      params.Avatar,
		DeptIds:     params.DeptIds,
	})
}

// saveAdminUserMutation 执行管理员用户变更操作（新增/更新）。
// 处理逻辑：
// 1. 验证用户是否存在（更新时）
// 2. 应用字段变更（创建/更新场景分别处理）
// 3. 验证唯一字段（用户名、手机号、邮箱）
// 4. 事务保存：用户数据、Token 撤销（密码变更/禁用时）、部门绑定
// 5. 同步用户权限缓存
func (s *AdminUserService) saveAdminUserMutation(params *adminUserEditParams) error {
	title := "新增"
	excludeID := uint(0)
	oldStatus := uint8(0)
	adminUserModel := model.NewAdminUsers()

	if params.Id > 0 {
		title = "更新"
		if err := adminUserModel.GetById(params.Id); err != nil {
			return e.NewBusinessError(e.UserDoesNotExist)
		}
		excludeID = params.Id
		oldStatus = adminUserModel.Status
	}

	var err error
	if params.Id > 0 {
		err = s.applyUpdateFields(adminUserModel, params)
	} else {
		err = s.applyCreateFields(adminUserModel, params)
	}
	if err != nil {
		return err
	}

	db, err := adminUserModel.GetDB()
	if err != nil {
		return e.NewBusinessError(e.FAILURE, title+"用户失败，请重试！")
	}

	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		adminUserModel.SetDB(tx)

		validateParams := map[string]interface{}{
			"username":     params.Username,
			"phone_number": params.PhoneNumber,
			"country_code": params.CountryCode,
			"email":        params.Email,
		}
		if err := s.validateUniqueFieldsWithLock(tx, validateParams, excludeID); err != nil {
			return err
		}
		if err := adminUserModel.Save(); err != nil {
			return err
		}

		if params.Id > 0 && params.Status != nil &&
			oldStatus == model.AdminUserStatusEnabled &&
			*params.Status == model.AdminUserStatusDisabled {
			s.revokeUserTokens(tx, adminUserModel.ID, model.RevokedCodeSystemForce, "系统强制登出（账号被封）")
		}
		if params.Id > 0 && params.Password != nil && *params.Password != "" {
			s.revokeUserTokens(tx, adminUserModel.ID, model.RevokedCodePasswordChangeAdmin, "管理员修改密码")
		}

		if params.DeptIds != nil {
			deptIDs := access.UniqueUintSlice(*params.DeptIds)
			return s.BindDept(adminUserModel.ID, deptIDs, tx)
		}

		return access.NewPermissionSyncCoordinator().SyncUser(adminUserModel.ID, tx)
	})
	if err := s.handleMutationError(err, title+"用户失败，请重试！"); err != nil {
		return err
	}
	return access.NewPermissionSyncCoordinator().ReloadPolicyCacheWithMessage(title + "用户后刷新权限缓存失败，请重试！")
}

// applyUpdateFields 应用更新场景的字段变更。
// 只更新非 nil 指针字段，支持部分更新语义。
// 特殊处理：
// - 超级管理员不可修改密码
// - 密码相同时拒绝更新
// - 超级管理员不可禁用
func (s *AdminUserService) applyUpdateFields(adminUserModel *model.AdminUser, params *adminUserEditParams) error {
	if params.Username != nil {
		adminUserModel.Username = *params.Username
	}
	if params.Nickname != nil {
		adminUserModel.Nickname = *params.Nickname
	}
	if params.Password != nil && *params.Password != "" {
		if adminUserModel.ID == global.SuperAdminId {
			return e.NewBusinessError(e.SuperAdminCannotModify)
		}
		if utils2.ComparePasswords(adminUserModel.Password, *params.Password) {
			return e.NewBusinessError(e.SamePassword)
		}
		if err := setHashedPassword(adminUserModel, *params.Password); err != nil {
			return err
		}
	}
	if params.PhoneNumber != nil {
		adminUserModel.PhoneNumber = *params.PhoneNumber
	}
	if params.CountryCode != nil {
		adminUserModel.CountryCode = *params.CountryCode
	} else if params.PhoneNumber != nil {
		adminUserModel.CountryCode = global.ChinaCountryCode
	}
	if params.Email != nil {
		adminUserModel.Email = *params.Email
	}
	if params.Status != nil {
		if *params.Status == model.AdminUserStatusDisabled && adminUserModel.ID == global.SuperAdminId {
			return e.NewBusinessError(e.SuperAdminCannotDisable)
		}
		adminUserModel.Status = *params.Status
	}
	if params.Avatar != nil {
		adminUserModel.Avatar = *params.Avatar
	}
	if params.PhoneNumber != nil || params.CountryCode != nil {
		adminUserModel.FullPhoneNumber = adminUserModel.CountryCode + adminUserModel.PhoneNumber
	}
	return nil
}

// applyCreateFields 应用新增场景的字段填充。
// 必填字段：用户名、昵称
// 默认值：国家代码默认为中国
func (s *AdminUserService) applyCreateFields(adminUserModel *model.AdminUser, params *adminUserEditParams) error {
	if params.Username == nil || *params.Username == "" {
		return e.NewBusinessError(e.UsernameRequired)
	}
	if params.Nickname == nil || *params.Nickname == "" {
		return e.NewBusinessError(e.NicknameRequired)
	}

	adminUserModel.Username = *params.Username
	adminUserModel.Nickname = *params.Nickname
	if params.PhoneNumber != nil {
		adminUserModel.PhoneNumber = *params.PhoneNumber
	}
	if params.CountryCode != nil {
		adminUserModel.CountryCode = *params.CountryCode
	} else {
		adminUserModel.CountryCode = global.ChinaCountryCode
	}
	if params.Email != nil {
		adminUserModel.Email = *params.Email
	}
	if params.Password != nil && *params.Password != "" {
		if err := setHashedPassword(adminUserModel, *params.Password); err != nil {
			return err
		}
	}
	if params.Avatar != nil {
		adminUserModel.Avatar = *params.Avatar
	}
	if params.Status != nil {
		adminUserModel.Status = *params.Status
	}
	adminUserModel.FullPhoneNumber = adminUserModel.CountryCode + adminUserModel.PhoneNumber
	return nil
}

// setHashedPassword 对用户密码进行哈希处理后设置到模型。
func setHashedPassword(adminUserModel *model.AdminUser, plainPassword string) error {
	passwordHash, err := utils2.PasswordHash(plainPassword)
	if err != nil {
		return e.NewBusinessError(e.PasswordProcessFailed)
	}
	adminUserModel.Password = passwordHash
	return nil
}

// UpdateProfile 更新个人资料。
func (s *AdminUserService) UpdateProfile(uid uint, params *form.UpdateProfile) error {
	adminUserModel := model.NewAdminUsers()
	err := adminUserModel.GetById(uid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return e.NewBusinessError(e.UserDoesNotExist)
		}
		return err
	}

	passwordChanged := params.Password != nil && *params.Password != ""
	hasUpdate := params.Nickname != nil ||
		passwordChanged ||
		params.PhoneNumber != nil ||
		params.CountryCode != nil ||
		params.Email != nil ||
		params.Avatar != nil
	if !hasUpdate {
		return nil
	}

	editParams := &adminUserEditParams{
		Id:          uid,
		Nickname:    params.Nickname,
		Password:    params.Password,
		PhoneNumber: params.PhoneNumber,
		CountryCode: params.CountryCode,
		Email:       params.Email,
		Avatar:      params.Avatar,
	}
	if err := s.applyUpdateFields(adminUserModel, editParams); err != nil {
		return err
	}

	db, err := adminUserModel.GetDB()
	if err != nil {
		return e.NewBusinessError(e.UpdateUserFailed)
	}
	return s.handleMutationError(access.RunInTransaction(db, func(tx *gorm.DB) error {
		adminUserModel.SetDB(tx)

		validateParams := map[string]interface{}{
			"phone_number": params.PhoneNumber,
			"country_code": params.CountryCode,
			"email":        params.Email,
		}
		if err := s.validateUniqueFieldsWithLock(tx, validateParams, uid); err != nil {
			return err
		}
		if err := adminUserModel.Save(); err != nil {
			return err
		}
		if passwordChanged {
			s.revokeUserTokens(tx, uid, model.RevokedCodePasswordChangeSelf, "用户自己修改密码")
		}
		return nil
	}), "更新个人资料失败，请重试！")
}

// validateUniqueFieldsWithLock 验证唯一字段（用户名、手机号、邮箱），使用数据库锁防止并发冲突。
// 参数：
//   - tx: 事务实例
//   - params: 待验证的字段值 map
//   - excludeId: 排除的当前用户 ID（更新场景）
func (s *AdminUserService) validateUniqueFieldsWithLock(tx *gorm.DB, params map[string]interface{}, excludeId uint) error {
	checkModel := model.NewAdminUsers()
	checkModel.SetDB(tx)

	// 验证用户名唯一性
	if usernameVal, ok := params["username"]; ok {
		if username, ok := usernameVal.(*string); ok && username != nil && *username != "" {
			exists, err := checkModel.ExistsWithLockExcludeId("username", *username, excludeId)
			if err != nil {
				return err
			}
			if exists {
				return e.NewBusinessError(e.UserExists, fmt.Sprintf("用户名 %s 已存在", *username))
			}
		}
	}

	// 验证手机号唯一性（使用完整手机号：国家代码 + 手机号）
	if phoneNumberVal, ok := params["phone_number"]; ok {
		if phoneNumber, ok := phoneNumberVal.(*string); ok && phoneNumber != nil && *phoneNumber != "" {
			countryCode := global.ChinaCountryCode
			if countryCodeVal, ok := params["country_code"]; ok {
				if cc, ok := countryCodeVal.(*string); ok && cc != nil && *cc != "" {
					countryCode = *cc
				}
			}
			fullPhoneNumber := countryCode + *phoneNumber
			if fullPhoneNumber != "" {
				exists, err := checkModel.ExistsWithLockExcludeId("full_phone_number", fullPhoneNumber, excludeId)
				if err != nil {
					return err
				}
				if exists {
					return e.NewBusinessError(e.PhoneNumberExists, fmt.Sprintf("手机号 %s 已存在", fullPhoneNumber))
				}
			}
		}
	}

	// 验证邮箱唯一性
	if emailVal, ok := params["email"]; ok {
		if email, ok := emailVal.(*string); ok && email != nil && *email != "" {
			exists, err := checkModel.ExistsWithLockExcludeId("email", *email, excludeId)
			if err != nil {
				return err
			}
			if exists {
				return e.NewBusinessError(e.EmailExists, fmt.Sprintf("邮箱 %s 已存在", *email))
			}
		}
	}

	return nil
}

// Delete 删除用户。
func (s *AdminUserService) Delete(id uint) error {
	if id == global.SuperAdminId {
		return e.NewBusinessError(e.SuperAdminCannotDelete)
	}

	adminUserModel := model.NewAdminUsers()
	adminUserDeptMap := model.NewAdminUserDeptMap()
	deptIds, err := model.ExtractColumnsByCondition[model.AdminUserDeptMap, *model.AdminUserDeptMap, uint](adminUserDeptMap, "dept_id", "uid = ?", id)
	if err != nil {
		return e.NewBusinessError(e.QueryUserDeptFailed)
	}

	db, err := adminUserModel.GetDB()
	if err != nil {
		return e.NewBusinessError(e.DeleteUserFailed)
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		adminUserModel.SetDB(tx)

		s.revokeUserTokens(tx, id, model.RevokedCodeOther, "管理员删除用户")

		if _, err := adminUserModel.DeleteByID(id); err != nil {
			return err
		}
		if len(deptIds) > 0 {
			if err := s.updateDeptUserNumber(deptIds, -1, tx); err != nil {
				return err
			}
		}
		return access.NewPermissionSyncCoordinator().ClearUser(id, tx)
	})
	if err != nil {
		return e.NewBusinessError(e.DeleteUserFailed)
	}

	return access.NewPermissionSyncCoordinator().ReloadPolicyCacheWithMessage("删除用户后刷新权限缓存失败")
}
