package admin

import (
	"errors"
	"fmt"

	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

// AdminUserService 授权服务
type AdminUserService struct {
	service.Base
}

// NewAdminUserService 创建管理员用户服务实例。
func NewAdminUserService() *AdminUserService {
	return &AdminUserService{}
}

func (s *AdminUserService) handleMutationError(err error, fallback string) error {
	if err == nil {
		return nil
	}

	var businessErr *e.BusinessError
	if errors.As(err, &businessErr) {
		return businessErr
	}

	return e.NewBusinessError(e.FAILURE, fallback)
}

func (s *AdminUserService) runMutationTransaction(db *gorm.DB, fallback string, fn func(tx *gorm.DB) error) error {
	return s.handleMutationError(access.RunInTransaction(db, fn), fallback)
}

func (s *AdminUserService) revokeUserTokens(tx *gorm.DB, userID uint, revokedCode uint8, revokedReason string) {
	loginService := auth.NewLoginService()
	if err := loginService.RevokeUserTokens(userID, revokedCode, revokedReason, tx); err != nil {
		log.Logger.Error("撤销用户token失败", zap.Error(err), zap.Uint("user_id", userID))
	}
}

// GetUserInfo 获取用户信息
func (s *AdminUserService) GetUserInfo(id uint) (*resources.AdminUserResources, error) {
	// 查询用户是否存在
	adminUsersModel := model.NewAdminUsers()
	// 显式 Preload RoleList 和 Department 关联，确保完整信息被正确加载
	db, err := adminUsersModel.GetDB()
	if err != nil {
		return nil, err
	}
	err = db.Preload("RoleList").Preload("Department").First(adminUsersModel, id).Error
	if err != nil {
		// 判断是是否不存在错误
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.FAILURE, "用户不存在")
		}
		return nil, err
	}

	return resources.NewAdminUserTransformer().ToStruct(adminUsersModel), nil
}

// GetUserMenuInfo 获取用户权限信息
func (s *AdminUserService) GetUserMenuInfo(id uint) (any, error) {
	// 查询用户是否存在
	adminUsersModel := model.NewAdminUsers()
	err := adminUsersModel.GetById(id)
	if err != nil {
		// 判断是是否不存在错误
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.FAILURE, "用户不存在")
		}
		return nil, err
	}
	condition, args := s.userMenuQuery(id == global.SuperAdminId, nil)
	if id != global.SuperAdminId {
		menuIDs, err := access.NewPermissionSyncCoordinator().AccessibleMenuIDs(id, true)
		if err != nil {
			return nil, err
		}
		condition, args = s.userMenuQuery(false, menuIDs)
	}

	// 获取菜单信息
	menus, err := model.ListE(model.NewMenu(), condition, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})
	if err != nil {
		return nil, err
	}

	return resources.NewMenuTreeTransformer().BuildTreeByNode(menus, 0), nil
}

// List 返回管理员分页列表。
func (s *AdminUserService) List(params *form.AdminUserList) *resources.Collection {
	conditionStr, args := s.buildListCondition(params)
	adminUserModel := model.NewAdminUsers()

	// 构建查询参数
	ListOptionalParams := model.ListOptionalParams{
		OrderBy: "created_at desc, id desc",
		Preload: map[string]func(db *gorm.DB) *gorm.DB{
			"Department": nil,
		},
	}

	total, collection, err := model.ListPageE(adminUserModel, params.Page, params.PerPage, conditionStr, args, ListOptionalParams)
	if err != nil {
		log.Logger.Error("查询管理员列表失败", zap.Error(err))
		return resources.NewAdminUserTransformer().ToCollection(params.Page, params.PerPage, 0, nil)
	}
	return resources.NewAdminUserTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

func (s *AdminUserService) buildListCondition(params *form.AdminUserList) (string, []any) {
	qb := query_builder.New().
		AddLike("username", params.UserName).
		AddEq("id", zeroToNil(params.ID)).
		AddLike("nickname", params.NickName).
		AddLike("email", params.Email).
		AddLike("full_phone_number", params.PhoneNumber).
		AddEq("status", params.Status)

	if params.DeptId > 0 {
		qb.AddCondition(
			"EXISTS (SELECT 1 FROM admin_user_department_map WHERE admin_user_department_map.uid = admin_user.id AND admin_user_department_map.dept_id = ?)",
			params.DeptId,
		)
	}

	return qb.Build()
}

func zeroToNil(value uint) any {
	if value == 0 {
		return nil
	}
	return value
}

func (s *AdminUserService) userMenuQuery(isSuperAdmin bool, menuIDs []uint) (string, []any) {
	if isSuperAdmin {
		return "status = ?", []any{1}
	}
	if len(menuIDs) == 0 {
		return "status = ? AND is_auth = ?", []any{1, 0}
	}
	return "status = ? AND (is_auth = ? OR (is_auth = ? AND id IN (?)))", []any{1, 0, 1, menuIDs}
}

type adminUserEditParams struct {
	Id          uint
	Username    *string
	Nickname    *string
	Password    *string
	PhoneNumber *string
	CountryCode *string
	Email       *string
	Status      *uint8
	Avatar      *string
	DeptIds     []uint
}

// Create 新增管理员用户。
func (s *AdminUserService) Create(params *form.CreateAdminUser) error {
	return s.edit(&adminUserEditParams{
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
	return s.edit(&adminUserEditParams{
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

// edit 编辑用户
func (s *AdminUserService) edit(params *adminUserEditParams) error {
	adminUserModel := model.NewAdminUsers()
	title := "新增"
	where := ""
	var oldStatus uint8 // 记录原始状态，用于判断是否从启用变为禁用

	if params.Id > 0 {
		title = "更新"
		// 编辑模式：从数据库加载现有数据
		err := adminUserModel.GetById(params.Id)
		if err != nil {
			return e.NewBusinessError(e.UserDoesNotExist)
		}
		where = fmt.Sprintf(" AND id != %d", params.Id)

		// 记录原始状态，用于判断是否从启用变为禁用
		oldStatus = adminUserModel.Status

		// 只更新提交的字段（通过指针是否为 nil 判断）
		if params.Username != nil {
			adminUserModel.Username = *params.Username
		}
		if params.Nickname != nil {
			adminUserModel.Nickname = *params.Nickname
		}
		if params.Password != nil && *params.Password != "" {
			// 检查是否为系统默认超级管理员，系统默认超级管理员不允许修改密码
			if adminUserModel.ID == global.SuperAdminId {
				return e.NewBusinessError(e.SuperAdminCannotModify)
			}
			// 验证新密码不能与旧密码相同
			if utils2.ComparePasswords(adminUserModel.Password, *params.Password) {
				return e.NewBusinessError(e.SamePassword)
			}
			passwordHash, err := utils2.PasswordHash(*params.Password)
			if err != nil {
				return e.NewBusinessError(e.PasswordProcessFailed)
			}
			adminUserModel.Password = passwordHash
		}
		if params.PhoneNumber != nil {
			adminUserModel.PhoneNumber = *params.PhoneNumber
		}
		if params.CountryCode != nil {
			adminUserModel.CountryCode = *params.CountryCode
		} else if params.PhoneNumber != nil {
			// 如果只提交了手机号，使用默认国家代码
			adminUserModel.CountryCode = global.ChinaCountryCode
		}
		if params.Email != nil {
			adminUserModel.Email = *params.Email
		}
		if params.Status != nil {
			// 检查是否为系统默认超级管理员，系统默认超级管理员不允许被禁用
			if *params.Status == model.AdminUserStatusDisabled && adminUserModel.ID == global.SuperAdminId {
				return e.NewBusinessError(e.SuperAdminCannotDisable)
			}
			adminUserModel.Status = *params.Status
		}
		if params.Avatar != nil {
			adminUserModel.Avatar = *params.Avatar
		}

		// 更新完整手机号
		if params.PhoneNumber != nil || params.CountryCode != nil {
			adminUserModel.FullPhoneNumber = adminUserModel.CountryCode + adminUserModel.PhoneNumber
		}
	} else {
		// 创建模式：验证必填字段
		if params.Username == nil || *params.Username == "" {
			return e.NewBusinessError(e.UsernameRequired)
		}
		if params.Nickname == nil || *params.Nickname == "" {
			return e.NewBusinessError(e.NicknameRequired)
		}

		// 创建模式：按原来的逻辑处理
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
			passwordHash, err := utils2.PasswordHash(*params.Password)
			if err != nil {
				return e.NewBusinessError(e.PasswordProcessFailed)
			}
			adminUserModel.Password = passwordHash
		}
		if params.Avatar != nil {
			adminUserModel.Avatar = *params.Avatar
		}
		if params.Status != nil {
			adminUserModel.Status = *params.Status
		}
		adminUserModel.FullPhoneNumber = adminUserModel.CountryCode + adminUserModel.PhoneNumber
	}

	// 在事务内使用 SELECT FOR UPDATE 加锁检查唯一性（防止并发插入）
	db, err := model.GetDB()
	if err != nil {
		return e.NewBusinessError(e.FAILURE, title+"用户失败，请重试！")
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		adminUserModel.SetDB(tx)

		// 在事务内使用 SELECT FOR UPDATE 加锁检查唯一性
		validateParams := map[string]interface{}{
			"username":     params.Username,
			"phone_number": params.PhoneNumber,
			"country_code": params.CountryCode,
			"email":        params.Email,
		}
		err := s.validateUniqueFieldsWithLock(tx, validateParams, where)
		if err != nil {
			return err
		}

		err = adminUserModel.Save()
		if err != nil {
			return err
		}

		// 如果用户状态从启用变为禁用，撤销该用户所有未过期的token（在事务内执行）
		if params.Id > 0 && params.Status != nil {
			if oldStatus == model.AdminUserStatusEnabled && *params.Status == model.AdminUserStatusDisabled {
				s.revokeUserTokens(tx, adminUserModel.ID, model.RevokedCodeSystemForce, "系统强制登出（账号被封）")
			}
		}

		if params.Id > 0 && params.Password != nil && *params.Password != "" {
			s.revokeUserTokens(tx, adminUserModel.ID, model.RevokedCodePasswordChangeAdmin, "管理员修改密码")
		}

		if len(params.DeptIds) > 0 {
			err = s.BindDept(adminUserModel.ID, params.DeptIds, tx)
			if err != nil {
				return err
			}
		}

		return access.NewPermissionSyncCoordinator().SyncUser(adminUserModel.ID, tx)
	})

	if err := s.handleMutationError(err, title+"用户失败，请重试！"); err != nil {
		return err
	}

	return access.NewPermissionSyncCoordinator().ReloadPolicyCacheWithMessage(title + "用户后刷新权限缓存失败，请重试！")
}

// UpdateProfile 更新个人资料（只能更新自己的手机号、邮箱、密码、昵称）
func (s *AdminUserService) UpdateProfile(uid uint, params *form.UpdateProfile) error {
	adminUserModel := model.NewAdminUsers()

	// 获取当前用户信息
	err := adminUserModel.GetById(uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return e.NewBusinessError(e.UserDoesNotExist)
		}
		return err
	}

	// 检查是否有任何字段需要更新
	hasUpdate := false
	passwordChanged := false

	// 更新昵称
	if params.Nickname != nil {
		adminUserModel.Nickname = *params.Nickname
		hasUpdate = true
	}

	// 更新密码
	if params.Password != nil && *params.Password != "" {
		// 检查是否为系统默认超级管理员，系统默认超级管理员不允许修改密码
		if uid == global.SuperAdminId {
			return e.NewBusinessError(e.SuperAdminCannotModify)
		}
		// 验证新密码不能与旧密码相同
		if utils2.ComparePasswords(adminUserModel.Password, *params.Password) {
			return e.NewBusinessError(e.SamePassword)
		}
		passwordHash, err := utils2.PasswordHash(*params.Password)
		if err != nil {
			return e.NewBusinessError(e.PasswordProcessFailed)
		}
		adminUserModel.Password = passwordHash
		hasUpdate = true
		passwordChanged = true
	}

	// 更新手机号
	if params.PhoneNumber != nil {
		adminUserModel.PhoneNumber = *params.PhoneNumber
		hasUpdate = true
	}

	// 更新国家代码
	if params.CountryCode != nil {
		adminUserModel.CountryCode = *params.CountryCode
		hasUpdate = true
	} else if params.PhoneNumber != nil {
		// 如果只提交了手机号，使用默认国家代码
		adminUserModel.CountryCode = global.ChinaCountryCode
		hasUpdate = true
	}

	// 更新邮箱
	if params.Email != nil {
		adminUserModel.Email = *params.Email
		hasUpdate = true
	}

	// 更新头像（文件ID或外部链接）
	if params.Avatar != nil {
		adminUserModel.Avatar = *params.Avatar
		hasUpdate = true
	}

	// 如果没有需要更新的字段，直接返回成功
	if !hasUpdate {
		return nil
	}

	// 更新完整手机号
	if params.PhoneNumber != nil || params.CountryCode != nil {
		adminUserModel.FullPhoneNumber = adminUserModel.CountryCode + adminUserModel.PhoneNumber
	}

	// 在事务内验证唯一性并更新
	db, err := model.GetDB()
	if err != nil {
		return e.NewBusinessError(e.UpdateUserFailed)
	}
	err = s.runMutationTransaction(db, "更新个人资料失败，请重试！", func(tx *gorm.DB) error {
		adminUserModel.SetDB(tx)

		// 使用 validateUniqueFieldsWithLock 验证唯一性（排除自己）
		where := fmt.Sprintf(" AND id != %d", uid)
		validateParams := map[string]interface{}{
			"phone_number": params.PhoneNumber,
			"country_code": params.CountryCode,
			"email":        params.Email,
		}
		err := s.validateUniqueFieldsWithLock(tx, validateParams, where)
		if err != nil {
			return err
		}

		if err := adminUserModel.Save(); err != nil {
			return err
		}

		if passwordChanged {
			s.revokeUserTokens(tx, uid, model.RevokedCodePasswordChangeSelf, "用户自己修改密码")
		}

		return nil
	})

	return err
}

// BindDept 绑定部门
func (s *AdminUserService) BindDept(uid uint, deptId []uint, tx ...*gorm.DB) (err error) {
	var dbTx *gorm.DB
	if len(tx) > 0 {
		dbTx = tx[0]
	} else {
		dbTx, err = model.GetDB()
		if err != nil {
			return err
		}
	}

	adminUserDeptMap := model.NewAdminUserDeptMap()
	adminUserDeptMap.SetDB(dbTx)

	// 获取该用户现有的所有部门关联（只查询 dept_id 字段）
	existingIds, err := model.ExtractColumnsByCondition[model.AdminUserDeptMap, *model.AdminUserDeptMap, uint](adminUserDeptMap, "dept_id", "uid = ?", uid)
	if err != nil {
		return err
	}

	// 一次性获取删除、新增和剩余列表
	toDelete, toAdd, remainingList := utils.CalculateChanges(existingIds, deptId)

	// 删除不再需要的关联
	if len(toDelete) > 0 {
		if err := adminUserDeptMap.DeleteWhere(
			"uid = ? AND dept_id IN (?)",
			[]any{uid, toDelete}...,
		); err != nil {
			return err
		}
		// 更新被删除部门的用户数量（减1）
		if err := s.updateDeptUserNumber(toDelete, -1, dbTx); err != nil {
			return err
		}
	}

	// 批量创建新关联
	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(deptId uint, _ int) *model.AdminUserDeptMap {
			return &model.AdminUserDeptMap{
				DeptId: deptId,
				Uid:    uid,
			}
		})
		if err := adminUserDeptMap.CreateBatch(newMappings); err != nil {
			return err
		}
		// 更新新增部门的用户数量（加1）
		if err := s.updateDeptUserNumber(toAdd, 1, dbTx); err != nil {
			return err
		}
	}

	_ = remainingList
	return access.NewPermissionSyncCoordinator().SyncUser(uid, tx...)
}

// updateDeptUserNumber 更新部门的用户数量
func (s *AdminUserService) updateDeptUserNumber(deptIds []uint, delta int, tx *gorm.DB) error {
	if len(deptIds) == 0 {
		return nil
	}

	deptModel := model.NewDepartment()
	deptModel.SetDB(tx)

	// 使用 SQL 表达式更新用户数量，避免并发问题
	// 如果 delta > 0，则增加；如果 delta < 0，则减少
	var updateExpr string
	if delta < 0 {
		// 确保 user_number 不会小于 0
		updateExpr = fmt.Sprintf("GREATEST(user_number + %d, 0)", delta)
	} else {
		updateExpr = fmt.Sprintf("user_number + %d", delta)
	}

	return tx.Model(deptModel).
		Where("id IN (?)", deptIds).
		Update("user_number", gorm.Expr(updateExpr)).Error
}

// validateUniqueFieldsWithLock 在事务内使用 SELECT FOR UPDATE 加锁检查唯一性
// 此方法在事务内调用，使用行锁防止并发插入重复数据
// 注意：对于不存在的记录，SELECT FOR UPDATE 会在索引上加锁，防止并发插入
// params map 的 key 支持: "username", "phone_number", "country_code", "email"
// 值类型为 *string，如果为 nil 或空字符串则跳过验证
func (s *AdminUserService) validateUniqueFieldsWithLock(tx *gorm.DB, params map[string]interface{}, where string) error {
	checkModel := model.NewAdminUsers()

	// 验证用户名唯一性（使用 SELECT FOR UPDATE 加锁）
	// 即使记录不存在，也会在唯一索引上加锁，防止并发插入
	if usernameVal, ok := params["username"]; ok {
		if username, ok := usernameVal.(*string); ok && username != nil && *username != "" {
			var exists bool
			err := tx.Model(checkModel).
				Select("1").
				Where("username = ? AND deleted_at = 0"+where, *username).
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Limit(1).
				Scan(&exists).Error
			if err != nil {
				return err
			}
			if exists {
				return e.NewBusinessError(e.UserExists, fmt.Sprintf("用户名 %s 已存在", *username))
			}
		}
	}

	// 验证手机号唯一性（使用 SELECT FOR UPDATE 加锁）
	// 即使记录不存在，也会在索引上加锁，防止并发插入
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
				var exists bool
				err := tx.Model(checkModel).
					Select("1").
					Where("full_phone_number = ? AND deleted_at = 0"+where, fullPhoneNumber).
					Clauses(clause.Locking{Strength: "UPDATE"}).
					Limit(1).
					Scan(&exists).Error
				if err != nil {
					return err
				}
				if exists {
					return e.NewBusinessError(e.PhoneNumberExists, fmt.Sprintf("手机号 %s 已存在", fullPhoneNumber))
				}
			}
		}
	}

	// 验证邮箱唯一性（使用 SELECT FOR UPDATE 加锁）
	// 即使记录不存在，也会在索引上加锁，防止并发插入
	if emailVal, ok := params["email"]; ok {
		if email, ok := emailVal.(*string); ok && email != nil && *email != "" {
			var exists bool
			err := tx.Model(checkModel).
				Select("1").
				Where("email = ? AND deleted_at = 0"+where, *email).
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Limit(1).
				Scan(&exists).Error
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

// Delete 删除用户
func (s *AdminUserService) Delete(id uint) error {
	if id == 1 {
		return e.NewBusinessError(e.SuperAdminCannotDelete)
	}

	adminUserModel := model.NewAdminUsers()

	// 在删除前获取用户所属的部门列表，用于更新部门用户数量
	adminUserDeptMap := model.NewAdminUserDeptMap()
	deptIds, err := model.ExtractColumnsByCondition[model.AdminUserDeptMap, *model.AdminUserDeptMap, uint](adminUserDeptMap, "dept_id", "uid = ?", id)
	if err != nil {
		return e.NewBusinessError(e.QueryUserDeptFailed)
	}

	// 使用事务确保数据一致性
	db, err := adminUserModel.GetDB()
	if err != nil {
		return e.NewBusinessError(e.DeleteUserFailed)
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		adminUserModel.SetDB(tx)

		// 删除用户
		_, err := adminUserModel.DeleteByID(id)
		if err != nil {
			return err
		}

		// 更新相关部门的用户数量（减1）
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

// BindRole 绑定角色
func (s *AdminUserService) BindRole(params *form.BindRole) error {
	adminUserModel := model.NewAdminUsers()
	err := adminUserModel.GetById(params.Id)
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "用户不存在")
	}

	// 检查角色是否存在
	// 判断是否有角色关联权限, 如果有则判断接口是否存在，只保留存在的接口ID
	ids, err := model.VerifyExistingIDs(model.NewRole(), params.RoleIds)
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "判断角色是否存在失败")
	}
	if err := access.NewSystemDefaultsService().RequireSuperAdminRoleForUser(adminUserModel.ID, ids); err != nil {
		return err
	}

	// 执行绑定角色事务
	db, err := model.GetDB()
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "绑定角色失败")
	}
	err = access.NewPermissionSyncCoordinator().RunAfterCommit(db, "绑定角色后刷新权限缓存失败", func(tx *gorm.DB) error {
		return s.updateAdminUserRole(adminUserModel.ID, ids, tx)
	})

	if err != nil {
		return e.NewBusinessError(e.FAILURE, "绑定角色失败")
	}
	return nil
}

// updateMenuPermissions 更新用户角色到关联中间表
func (s *AdminUserService) updateAdminUserRole(uid uint, roleIds []uint, tx ...*gorm.DB) error {
	if err := access.NewSystemDefaultsService().RequireSuperAdminRoleForUser(uid, roleIds); err != nil {
		return err
	}

	adminUserRoleMap := model.NewAdminUserRoleMap()
	if len(tx) > 0 {
		adminUserRoleMap.SetDB(tx[0])
	}
	existingIds, err := model.ExtractColumnsByCondition[model.AdminUserRoleMap, *model.AdminUserRoleMap, uint](adminUserRoleMap, "role_id", "uid = ?", uid)
	if err != nil {
		return err
	}
	// 2. 计算差集（一次性获取删除和新增列表）
	toDelete, toAdd, remainingList := utils.CalculateChanges(existingIds, roleIds)

	// 删除不再需要的关联
	if len(toDelete) > 0 {
		if err := adminUserRoleMap.DeleteWhere(
			"uid = ? AND role_id IN (?)",
			[]any{uid, toDelete}...,
		); err != nil {
			return err
		}
	}

	// 批量创建新关联
	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(roleId uint, _ int) *model.AdminUserRoleMap {
			return &model.AdminUserRoleMap{
				RoleId: roleId,
				Uid:    uid,
			}
		})
		if err := adminUserRoleMap.CreateBatch(newMappings); err != nil {
			return err
		}
	}

	_ = remainingList
	return access.NewPermissionSyncCoordinator().SyncUser(uid, tx...)
}
