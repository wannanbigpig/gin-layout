package admin

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

var loadAdminUserForSuperAdminCheck = func(id uint) (*model.AdminUser, error) {
	user := model.NewAdminUsers()
	if err := user.GetById(id); err != nil {
		return nil, err
	}
	return user, nil
}

// Create 新增管理员用户。
func (s *AdminUserService) Create(params *form.CreateAdminUser) error {
	return s.saveAdminUserMutation(&adminUserEditParams{
		Username:     params.Username,
		Nickname:     params.Nickname,
		Password:     params.Password,
		PhoneNumber:  params.PhoneNumber,
		CountryCode:  params.CountryCode,
		Email:        params.Email,
		Status:       params.Status,
		IsSuperAdmin: params.IsSuperAdmin,
		Avatar:       params.Avatar,
		DeptIds:      params.DeptIds,
	})
}

// Update 更新管理员用户。
func (s *AdminUserService) Update(params *form.UpdateAdminUser) error {
	if err := s.requireUserInDataScope(params.Id); err != nil {
		return err
	}
	return s.saveAdminUserMutation(&adminUserEditParams{
		Id:           params.Id,
		Username:     params.Username,
		Nickname:     params.Nickname,
		Password:     params.Password,
		PhoneNumber:  params.PhoneNumber,
		CountryCode:  params.CountryCode,
		Email:        params.Email,
		Status:       params.Status,
		IsSuperAdmin: params.IsSuperAdmin,
		Avatar:       params.Avatar,
		DeptIds:      params.DeptIds,
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
	if params.DeptIds != nil {
		if err := s.requireDepartmentsInDataScope(*params.DeptIds); err != nil {
			return err
		}
	}

	mutationFailedCode := e.CreateUserFailed
	excludeID := uint(0)
	adminUserModel := model.NewAdminUsers()
	isUpdate := params.Id > 0
	if isUpdate {
		mutationFailedCode = e.UpdateUserFailed
		excludeID = params.Id
	} else {
		// 先返回明确的权限错误，事务内会再次锁定校验以防授权状态并发变化。
		if err := s.authorizeAdminUserMutation(adminUserModel, params); err != nil {
			return err
		}
		if err := s.applyCreateFields(adminUserModel, params); err != nil {
			return err
		}
	}

	db, err := adminUserModel.GetDB()
	if err != nil {
		return e.NewBusinessError(mutationFailedCode)
	}

	pendingRevocations := make([]userTokenRevocation, 0, 2)
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		adminUserModel.SetDB(tx)
		oldStatus := adminUserModel.Status
		oldIsSuperAdmin := adminUserModel.IsSuperAdmin
		oldAvatar := adminUserModel.Avatar
		if isUpdate {
			if err := loadAdminUserForUpdate(tx, adminUserModel, params.Id); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return e.NewBusinessError(e.UserDoesNotExist)
				}
				return err
			}
			oldStatus = adminUserModel.Status
			oldIsSuperAdmin = adminUserModel.IsSuperAdmin
			oldAvatar = adminUserModel.Avatar
			if err := s.authorizeAdminUserMutation(adminUserModel, params, tx); err != nil {
				return err
			}
			if err := s.applyUpdateFields(adminUserModel, params); err != nil {
				return err
			}
		} else if err := s.authorizeAdminUserMutation(adminUserModel, params, tx); err != nil {
			return err
		}

		validateParams := map[string]interface{}{
			"username": params.Username,
			"email":    params.Email,
		}
		if (params.PhoneNumber != nil || params.CountryCode != nil) && adminUserModel.PhoneNumber != "" {
			validateParams["full_phone_number"] = adminUserModel.FullPhoneNumber
		}
		if err := s.validateUniqueFieldsWithLock(tx, validateParams, excludeID); err != nil {
			return err
		}
		if isUpdate {
			if err := tx.Model(&model.AdminUser{}).
				Where("id = ? AND deleted_at = 0", adminUserModel.ID).
				Updates(adminUserUpdateValues(adminUserModel, params)).Error; err != nil {
				return err
			}
		} else if err := adminUserModel.Save(); err != nil {
			return err
		}
		if params.Avatar != nil {
			refService := service.NewFileReferenceService(tx)
			if oldAvatar != "" && oldAvatar != adminUserModel.Avatar {
				if err := refService.ReleaseReference(oldAvatar, "admin_user", adminUserModel.ID, "avatar"); err != nil {
					return err
				}
			}
			if adminUserModel.Avatar != "" {
				if err := refService.BindReference(adminUserModel.Avatar, "admin_user", adminUserModel.ID, "avatar"); err != nil {
					return err
				}
			}
		}

		if revocation := adminUserMutationRevocation(adminUserModel.ID, oldStatus, oldIsSuperAdmin, params); revocation != nil {
			if err := s.prepareUserTokenRevocation(tx, revocation); err != nil {
				return err
			}
			pendingRevocations = append(pendingRevocations, *revocation)
		}

		if params.DeptIds != nil {
			deptIDs := access.UniqueUintSlice(*params.DeptIds)
			return s.BindDept(adminUserModel.ID, deptIDs, tx)
		}

		return access.NewPermissionSyncCoordinator().SyncUser(adminUserModel.ID, tx)
	})
	if err := s.handleMutationError(err, mutationFailedCode); err != nil {
		return err
	}
	revokeErr := s.revokeUserTokensAfterCommit(pendingRevocations)
	reloadErr := access.NewPermissionSyncCoordinator().ReloadPolicyCacheWithCode(mutationFailedCode)
	if reloadErr != nil {
		return reloadErr
	}
	if params.Id > 0 && params.DeptIds != nil {
		access.NewDataScopeFilter().InvalidateCache(context.Background(), adminUserModel.ID)
	}
	return s.handleMutationError(revokeErr, mutationFailedCode)
}

func loadAdminUserForUpdate(tx *gorm.DB, user *model.AdminUser, id uint) error {
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND deleted_at = 0", id).
		First(user).Error
}

func adminUserUpdateValues(user *model.AdminUser, params *adminUserEditParams) map[string]any {
	values := make(map[string]any)
	if params.Username != nil {
		values["username"] = user.Username
	}
	if params.Nickname != nil {
		values["nickname"] = user.Nickname
	}
	if params.Password != nil && *params.Password != "" {
		values["password"] = user.Password
	}
	if params.PhoneNumber != nil {
		values["phone_number"] = user.PhoneNumber
	}
	if params.PhoneNumber != nil || params.CountryCode != nil {
		values["country_code"] = user.CountryCode
		values["full_phone_number"] = user.FullPhoneNumber
	}
	if params.Email != nil {
		values["email"] = user.Email
	}
	if params.Status != nil {
		values["status"] = user.Status
	}
	if params.IsSuperAdmin != nil {
		values["is_super_admin"] = user.IsSuperAdmin
	}
	if params.Avatar != nil {
		values["avatar"] = user.Avatar
	}
	return values
}

func adminUserMutationRevocation(userID uint, oldStatus, oldIsSuperAdmin uint8, params *adminUserEditParams) *userTokenRevocation {
	if params.Id == 0 {
		return nil
	}
	if params.Status != nil && oldStatus == model.AdminUserStatusEnabled && *params.Status == model.AdminUserStatusDisabled {
		return &userTokenRevocation{userID: userID, revokedCode: model.RevokedCodeSystemForce, revokedReason: "系统强制登出（账号被封）"}
	}
	if params.Password != nil && *params.Password != "" {
		return &userTokenRevocation{userID: userID, revokedCode: model.RevokedCodePasswordChangeAdmin, revokedReason: "管理员修改密码"}
	}
	if params.IsSuperAdmin != nil && oldIsSuperAdmin != *params.IsSuperAdmin {
		return &userTokenRevocation{userID: userID, revokedCode: model.RevokedCodeOther, revokedReason: "管理员权限变更"}
	}
	return nil
}

// authorizeAdminUserMutation 保护已有超级管理员及超级身份授予，普通账号创建无需超级权限。
func (s *AdminUserService) authorizeAdminUserMutation(target *model.AdminUser, params *adminUserEditParams, tx ...*gorm.DB) error {
	protectedTarget := params.Id > 0 && (target.ID == global.SuperAdminId || target.IsSuperAdmin == global.Yes)
	changesSuperAdmin := params.IsSuperAdmin != nil && target.IsSuperAdmin != *params.IsSuperAdmin
	if (protectedTarget || changesSuperAdmin) && !s.actorCanManageSuperAdmin(tx...) {
		return e.NewBusinessError(e.AuthorizationErr)
	}
	return nil
}

func (s *AdminUserService) actorCanManageSuperAdmin(tx ...*gorm.DB) bool {
	actorID := s.GetAdminUserId()
	if actorID == global.SuperAdminId {
		return true
	}
	if actorID == 0 {
		return false
	}
	if currentTx := access.FirstTx(tx); currentTx != nil {
		var actor struct{ IsSuperAdmin uint8 }
		if err := currentTx.Model(&model.AdminUser{}).
			Select("is_super_admin").
			Clauses(clause.Locking{Strength: "SHARE"}).
			Where("id = ? AND deleted_at = 0", actorID).
			Take(&actor).Error; err != nil {
			return false
		}
		return actor.IsSuperAdmin == global.Yes
	}
	actor := model.NewAdminUsers()
	loaded, err := loadAdminUserForSuperAdminCheck(actorID)
	if err != nil {
		return false
	}
	if loaded != nil {
		actor = loaded
	}
	return actor.IsSuperAdmin == global.Yes
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
	} else if params.PhoneNumber != nil && adminUserModel.CountryCode == "" {
		// 仅在记录本身尚无国家区号时才默认中国；否则保留既有区号，
		// 避免「只改手机号、未传 country_code」时把非中国区号误改成 +86。
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
	if params.IsSuperAdmin != nil {
		if *params.IsSuperAdmin == global.No && adminUserModel.ID == global.SuperAdminId {
			return e.NewBusinessError(e.SuperAdminCannotModify)
		}
		adminUserModel.IsSuperAdmin = *params.IsSuperAdmin
	}
	if params.Avatar != nil {
		adminUserModel.Avatar = *params.Avatar
	}
	if params.PhoneNumber != nil || params.CountryCode != nil {
		if adminUserModel.PhoneNumber == "" {
			adminUserModel.FullPhoneNumber = ""
		} else {
			adminUserModel.FullPhoneNumber = adminUserModel.CountryCode + adminUserModel.PhoneNumber
		}
	}
	return nil
}

// applyCreateFields 应用新增场景的字段填充。
// 必填字段：用户名、昵称、密码
// 默认值：国家代码默认为中国
func (s *AdminUserService) applyCreateFields(adminUserModel *model.AdminUser, params *adminUserEditParams) error {
	if params.Username == nil || *params.Username == "" {
		return e.NewBusinessError(e.UsernameRequired)
	}
	if params.Nickname == nil || *params.Nickname == "" {
		return e.NewBusinessError(e.NicknameRequired)
	}
	if params.Password == nil || *params.Password == "" {
		return e.NewBusinessError(e.PasswordRequired)
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
	if err := setHashedPassword(adminUserModel, *params.Password); err != nil {
		return err
	}
	if params.Avatar != nil {
		adminUserModel.Avatar = *params.Avatar
	}
	if params.Status != nil {
		adminUserModel.Status = *params.Status
	}
	if params.IsSuperAdmin != nil {
		adminUserModel.IsSuperAdmin = *params.IsSuperAdmin
	}
	if adminUserModel.PhoneNumber != "" {
		adminUserModel.FullPhoneNumber = adminUserModel.CountryCode + adminUserModel.PhoneNumber
	}
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

	adminUserModel := model.NewAdminUsers()
	editParams := &adminUserEditParams{
		Id:          uid,
		Nickname:    params.Nickname,
		Password:    params.Password,
		PhoneNumber: params.PhoneNumber,
		CountryCode: params.CountryCode,
		Email:       params.Email,
		Avatar:      params.Avatar,
	}
	db, err := adminUserModel.GetDB()
	if err != nil {
		return e.NewBusinessError(e.UpdateUserFailed)
	}
	pendingRevocations := make([]userTokenRevocation, 0, 1)
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		adminUserModel.SetDB(tx)
		if err := loadAdminUserForUpdate(tx, adminUserModel, uid); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return e.NewBusinessError(e.UserDoesNotExist)
			}
			return err
		}
		oldAvatar := adminUserModel.Avatar
		if err := s.applyUpdateFields(adminUserModel, editParams); err != nil {
			return err
		}

		validateParams := map[string]interface{}{
			"email": params.Email,
		}
		if (params.PhoneNumber != nil || params.CountryCode != nil) && adminUserModel.PhoneNumber != "" {
			validateParams["full_phone_number"] = adminUserModel.FullPhoneNumber
		}
		if err := s.validateUniqueFieldsWithLock(tx, validateParams, uid); err != nil {
			return err
		}
		if err := tx.Model(&model.AdminUser{}).
			Where("id = ? AND deleted_at = 0", uid).
			Updates(adminUserUpdateValues(adminUserModel, editParams)).Error; err != nil {
			return err
		}
		if params.Avatar != nil {
			refService := service.NewFileReferenceService(tx)
			if oldAvatar != "" && oldAvatar != adminUserModel.Avatar {
				if err := refService.ReleaseReference(oldAvatar, "admin_user", uid, "avatar"); err != nil {
					return err
				}
			}
			if adminUserModel.Avatar != "" {
				if err := refService.BindReference(adminUserModel.Avatar, "admin_user", uid, "avatar"); err != nil {
					return err
				}
			}
		}
		if passwordChanged {
			revocation := &userTokenRevocation{
				userID:        uid,
				revokedCode:   model.RevokedCodePasswordChangeSelf,
				revokedReason: "用户自己修改密码",
			}
			if err := s.prepareUserTokenRevocation(tx, revocation); err != nil {
				return err
			}
			pendingRevocations = append(pendingRevocations, *revocation)
		}
		return nil
	})
	if err := s.handleMutationError(err, e.UpdateUserFailed); err != nil {
		return err
	}
	return s.handleMutationError(s.revokeUserTokensAfterCommit(pendingRevocations), e.UpdateUserFailed)
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
				return e.NewBusinessError(e.UserExists)
			}
		}
	}

	// 验证手机号唯一性（使用模型最终的完整手机号，覆盖仅修改国家代码的场景）。
	if fullPhoneNumberVal, ok := params["full_phone_number"]; ok {
		if fullPhoneNumber, ok := fullPhoneNumberVal.(string); ok && fullPhoneNumber != "" {
			exists, err := checkModel.ExistsWithLockExcludeId("full_phone_number", fullPhoneNumber, excludeId)
			if err != nil {
				return err
			}
			if exists {
				return e.NewBusinessError(e.PhoneNumberExists)
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
				return e.NewBusinessError(e.EmailExists)
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
	if err := s.requireUserInDataScope(id); err != nil {
		return err
	}

	adminUserModel := model.NewAdminUsers()
	adminUserDeptMap := model.NewAdminUserDeptMap()
	db, err := adminUserModel.GetDB()
	if err != nil {
		return e.NewBusinessError(e.DeleteUserFailed)
	}
	var deptIds []uint
	pendingRevocations := []userTokenRevocation{{
		userID:        id,
		revokedCode:   model.RevokedCodeOther,
		revokedReason: "管理员删除用户",
	}}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		adminUserModel.SetDB(tx)
		// 与 login/refresh 及其他账号安全变更保持 user -> session 的加锁顺序。
		if err := loadAdminUserForUpdate(tx, adminUserModel, id); err != nil {
			return err
		}
		if err := s.authorizeAdminUserMutation(adminUserModel, &adminUserEditParams{Id: id}, tx); err != nil {
			return err
		}
		adminUserDeptMap.SetDB(tx)
		var queryErr error
		deptIds, queryErr = model.ExtractColumnsByCondition[model.AdminUserDeptMap, *model.AdminUserDeptMap, uint](adminUserDeptMap, "dept_id", "uid = ?", id)
		if queryErr != nil {
			return e.NewBusinessError(e.QueryUserDeptFailed)
		}
		if err := s.prepareUserTokenRevocation(tx, &pendingRevocations[0]); err != nil {
			return err
		}

		if _, err := adminUserModel.DeleteByID(id); err != nil {
			return err
		}
		if err := service.NewFileReferenceService(tx).ReleaseReferencesByOwner("admin_user", id, "avatar"); err != nil {
			return err
		}
		if err := adminUserDeptMap.DeleteWhere("uid = ?", id); err != nil {
			return err
		}
		adminUserRoleMap := model.NewAdminUserRoleMap()
		adminUserRoleMap.SetDB(tx)
		if err := adminUserRoleMap.DeleteWhere("uid = ?", id); err != nil {
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

	revokeErr := s.revokeUserTokensAfterCommit(pendingRevocations)
	reloadErr := access.NewPermissionSyncCoordinator().ReloadPolicyCacheWithCode(e.DeleteUserFailed)
	if reloadErr != nil {
		return reloadErr
	}
	access.NewDataScopeFilter().InvalidateCache(context.Background(), id)
	return s.handleMutationError(revokeErr, e.DeleteUserFailed)
}
