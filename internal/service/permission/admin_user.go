package permission

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	casbinx "github.com/wannanbigpig/gin-layout/internal/pkg/utils/casbin"
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
func (s *AdminUserService) GetUserInfo(id uint) (*resources.AdminUserResources, error) {
	// 查询用户是否存在
	adminUsersModel := model.NewAdminUsers()
	// 显式 Preload RoleList 和 Department 关联，确保完整信息被正确加载
	err := adminUsersModel.DB().Preload("RoleList").Preload("Department").First(adminUsersModel, id).Error
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
	err := adminUsersModel.GetById(adminUsersModel, id)
	if err != nil {
		// 判断是是否不存在错误
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.FAILURE, "用户不存在")
		}
		return nil, err
	}
	var condition string
	var args []any

	// 不是超级管理员则根据他对应权限获取菜单
	if id != global.SuperAdminId {
		// 获取用户拥有的角色
		roles, err := s.getImplicitRolesForAdminUser(id)
		if err != nil {
			return nil, err
		}

		// 获取所有菜单ID，roles：["role:1","role:3","menu:1","menu:3"]
		menuSet := make(map[uint]struct{})
		for _, role := range roles {
			if idStr, ok := strings.CutPrefix(role, "menu:"); ok {
				if menuID, err := strconv.ParseUint(idStr, 10, 64); err == nil {
					menuSet[uint(menuID)] = struct{}{}
				}
			}
		}

		menuIDs := make([]uint, 0, len(menuSet))
		for menuID := range menuSet {
			menuIDs = append(menuIDs, menuID)
		}

		// 返回所有不需要鉴权的菜单（is_auth = 0）加上用户有权限的菜单（is_auth = 1 且 id IN menuIDs）
		// 如果用户没有任何菜单权限，只返回不需要鉴权的菜单
		if len(menuIDs) > 0 {
			condition = "status = ? AND (is_auth = ? OR (is_auth = ? AND id IN (?)))"
			args = []any{1, 0, 1, menuIDs}
		} else {
			// 用户没有任何权限，只返回不需要鉴权的菜单
			condition = "status = ? AND is_auth = ?"
			args = []any{1, 0}
		}
	} else {
		// 超级管理员返回所有启用的菜单
		condition = "status = ?"
		args = []any{1}
	}

	// 获取菜单信息
	menus := model.List(model.NewMenu(), condition, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})

	return resources.NewMenuTreeTransformer().BuildTreeByNode(menus, 0), nil
}

func (s *AdminUserService) List(params *form.AdminUserList) *resources.Collection {
	var conditions []string
	var args []any
	if params.UserName != "" {
		conditions = append(conditions, "username like ?")
		args = append(args, "%"+params.UserName+"%")
	}
	if params.ID != 0 {
		conditions = append(conditions, "id = ?")
		args = append(args, params.ID)
	}
	if params.NickName != "" {
		conditions = append(conditions, "nickname like ?")
		args = append(args, "%"+params.NickName+"%")
	}
	if params.Email != "" {
		conditions = append(conditions, "email like ?")
		args = append(args, "%"+params.Email+"%")
	}
	if params.PhoneNumber != "" {
		conditions = append(conditions, "full_phone_number like ?")
		args = append(args, "%"+params.PhoneNumber+"%")
	}
	if params.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, params.Status)
	}

	conditionStr := strings.Join(conditions, " AND ")
	adminUserModel := model.NewAdminUsers()

	// 构建查询参数
	ListOptionalParams := model.ListOptionalParams{
		OrderBy: "created_at desc, id desc",
		Preload: map[string]func(db *gorm.DB) *gorm.DB{
			"Department": nil,
		},
	}

	// 如果有部门过滤条件，使用 EXISTS 方式查询
	if params.DeptId > 0 {
		// 使用 EXISTS 子查询过滤部门
		conditions = append(conditions, "EXISTS (SELECT 1 FROM a_admin_user_department_map WHERE a_admin_user_department_map.uid = a_admin_user.id AND a_admin_user_department_map.dept_id = ?)")
		args = append(args, params.DeptId)
		conditionStr = strings.Join(conditions, " AND ")
	}

	total, collection := model.ListPage(adminUserModel, params.Page, params.PerPage, conditionStr, args, ListOptionalParams)
	return resources.NewAdminUserTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

// Edit 编辑用户
func (s *AdminUserService) Edit(params *form.EditAdminUser) error {
	adminUserModel := model.NewAdminUsers()
	title := "新增"
	where := ""
	var oldStatus int8 // 记录原始状态，用于判断是否从启用变为禁用

	if params.Id > 0 {
		title = "更新"
		// 编辑模式：从数据库加载现有数据
		err := adminUserModel.GetById(adminUserModel, params.Id)
		if err != nil {
			return e.NewBusinessError(e.FAILURE, "用户不存在")
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
				return e.NewBusinessError(e.FAILURE, "系统默认超级管理员不允许修改密码")
			}
			// 验证新密码不能与旧密码相同
			if utils2.ComparePasswords(adminUserModel.Password, *params.Password) {
				return e.NewBusinessError(e.FAILURE, "新密码不能与当前密码相同")
			}
			passwordHash, err := utils2.PasswordHash(*params.Password)
			if err != nil {
				return e.NewBusinessError(e.FAILURE, "密码处理失败")
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
			if *params.Status == int8(model.AdminUserStatusDisabled) && adminUserModel.ID == global.SuperAdminId {
				return e.NewBusinessError(e.FAILURE, "系统默认超级管理员不允许被禁用")
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
			return e.NewBusinessError(e.FAILURE, "用户名必填")
		}
		if params.Nickname == nil || *params.Nickname == "" {
			return e.NewBusinessError(e.FAILURE, "昵称必填")
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
				return e.NewBusinessError(e.FAILURE, "密码处理失败")
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
	err := model.DB().Transaction(func(tx *gorm.DB) error {
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

		err = adminUserModel.Save(adminUserModel)
		if err != nil {
			return err
		}

		// 如果用户状态从启用变为禁用，撤销该用户所有未过期的token（在事务内执行）
		if params.Id > 0 && params.Status != nil {
			if oldStatus == int8(model.AdminUserStatusEnabled) && *params.Status == int8(model.AdminUserStatusDisabled) {
				loginService := NewLoginService()
				if err := loginService.RevokeUserTokens(adminUserModel.ID, model.RevokedCodeSystemForce, "系统强制登出（账号被封）", tx); err != nil {
					log.Logger.Error("撤销用户token失败", zap.Error(err), zap.Uint("user_id", adminUserModel.ID))
					// 不阻断用户禁用操作，只记录错误日志
				}
			}
		}

		// 如果修改了密码，撤销该用户所有未过期的token（在事务内执行）
		// 注意：密码是加密存储的（bcrypt），每次加密结果都不同，所以只要提交了新密码就撤销token
		if params.Id > 0 && params.Password != nil && *params.Password != "" {
			loginService := NewLoginService()
			if err := loginService.RevokeUserTokens(adminUserModel.ID, model.RevokedCodePasswordChangeAdmin, "管理员修改密码", tx); err != nil {
				log.Logger.Error("撤销用户token失败", zap.Error(err), zap.Uint("user_id", adminUserModel.ID))
				// 不阻断密码修改操作，只记录错误日志
			}
		}

		// 绑定部门（创建时必绑定，编辑时如果提交了才绑定）
		if len(params.DeptIds) > 0 {
			err = s.BindDept(adminUserModel.ID, params.DeptIds, tx)
			if err != nil {
				return err
			}
		}

		return nil
	})

	// 如果正常提交，那么内存中的数据就和数据库中的数据一致了，无需再次加载，如果是回滚，那么内存中的数据就和数据库中的数据不一致了，需要再次加载
	if err != nil {
		_ = casbinx.GetEnforcer().LoadPolicy()

		// 如果是业务错误（如唯一性校验失败），直接返回明确的错误信息
		var businessErr *e.BusinessError
		if errors.As(err, &businessErr) {
			return businessErr
		}

		// 其他错误返回通用错误信息
		return e.NewBusinessError(e.FAILURE, title+"用户失败，请重试！")
	}

	return nil
}

// UpdateProfile 更新个人资料（只能更新自己的手机号、邮箱、密码、昵称）
func (s *AdminUserService) UpdateProfile(uid uint, params *form.UpdateProfile) error {
	adminUserModel := model.NewAdminUsers()

	// 获取当前用户信息
	err := adminUserModel.GetById(adminUserModel, uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return e.NewBusinessError(e.FAILURE, "用户不存在")
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
			return e.NewBusinessError(e.FAILURE, "系统默认超级管理员不允许修改密码")
		}
		// 验证新密码不能与旧密码相同
		if utils2.ComparePasswords(adminUserModel.Password, *params.Password) {
			return e.NewBusinessError(e.FAILURE, "新密码不能与当前密码相同")
		}
		passwordHash, err := utils2.PasswordHash(*params.Password)
		if err != nil {
			return e.NewBusinessError(e.FAILURE, "密码处理失败")
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
	err = model.DB().Transaction(func(tx *gorm.DB) error {
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

		// 更新用户信息
		err = adminUserModel.Save(adminUserModel)
		if err != nil {
			return err
		}

		// 如果修改了密码，撤销该用户所有未过期的token（在事务内执行）
		// 注意：密码是加密存储的（bcrypt），每次加密结果都不同，所以只要提交了新密码就撤销token
		if passwordChanged {
			loginService := NewLoginService()
			if err := loginService.RevokeUserTokens(uid, model.RevokedCodePasswordChangeSelf, "用户自己修改密码", tx); err != nil {
				log.Logger.Error("撤销用户token失败", zap.Error(err), zap.Uint("user_id", uid))
				// 不阻断密码修改操作，只记录错误日志
			}
		}

		return nil
	})

	if err != nil {
		// 如果是业务错误，直接返回
		var businessErr *e.BusinessError
		if errors.As(err, &businessErr) {
			return businessErr
		}
		// 其他错误返回通用错误信息
		return e.NewBusinessError(e.FAILURE, "更新个人资料失败，请重试！")
	}

	return nil
}

// BindDept 绑定部门
func (s *AdminUserService) BindDept(uid uint, deptId []uint, tx ...*gorm.DB) (err error) {
	var dbTx *gorm.DB
	if len(tx) > 0 {
		dbTx = tx[0]
	} else {
		dbTx = model.DB()
	}

	adminUsesDeptMap := model.NewAdminUsesDeptMap()
	adminUsesDeptMap.SetDB(dbTx)

	// 获取该用户现有的所有部门关联（只查询 dept_id 字段）
	existingIds, err := model.ExtractColumnsByCondition[model.AdminUsesDeptMap, *model.AdminUsesDeptMap, uint](adminUsesDeptMap, "dept_id", "uid = ?", uid)
	if err != nil {
		return err
	}

	// 一次性获取删除、新增和剩余列表
	toDelete, toAdd, remainingList := utils.CalculateChanges(existingIds, deptId)

	// 删除不再需要的关联
	if len(toDelete) > 0 {
		if err := adminUsesDeptMap.DeleteWithCondition(
			adminUsesDeptMap,
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
		newMappings := lo.Map(toAdd, func(deptId uint, _ int) *model.AdminUsesDeptMap {
			return &model.AdminUsesDeptMap{
				DeptId: deptId,
				Uid:    uid,
			}
		})
		if err := adminUsesDeptMap.BatchCreate(newMappings); err != nil {
			return err
		}
		// 更新新增部门的用户数量（加1）
		if err := s.updateDeptUserNumber(toAdd, 1, dbTx); err != nil {
			return err
		}
	}

	return s.editAdminUserPolicyRoles(uid, global.CasbinDeptPrefix, remainingList, tx...)
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
				return e.NewBusinessError(1, fmt.Sprintf("用户名 %s 已存在", *username))
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
					return e.NewBusinessError(1, fmt.Sprintf("手机号 %s 已存在", fullPhoneNumber))
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
				return e.NewBusinessError(1, fmt.Sprintf("邮箱 %s 已存在", *email))
			}
		}
	}

	return nil
}

// Delete 删除用户
func (s *AdminUserService) Delete(id uint) error {
	if id == 1 {
		return e.NewBusinessError(1, "系统默认超级管理员不允许删除")
	}

	adminUserModel := model.NewAdminUsers()

	// 在删除前获取用户所属的部门列表，用于更新部门用户数量
	adminUsesDeptMap := model.NewAdminUsesDeptMap()
	deptIds, err := model.ExtractColumnsByCondition[model.AdminUsesDeptMap, *model.AdminUsesDeptMap, uint](adminUsesDeptMap, "dept_id", "uid = ?", id)
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "查询用户部门关联失败")
	}

	// 使用事务确保数据一致性
	err = adminUserModel.DB().Transaction(func(tx *gorm.DB) error {
		adminUserModel.SetDB(tx)

		// 删除用户
		_, err := adminUserModel.Delete(adminUserModel, id)
		if err != nil {
			return err
		}

		// 更新相关部门的用户数量（减1）
		if len(deptIds) > 0 {
			if err := s.updateDeptUserNumber(deptIds, -1, tx); err != nil {
				return err
			}
		}

		// 删除用户的Casbin策略（传入空数组会自动删除所有策略）
		return s.editAdminUserPolicyRoles(id, global.CasbinRolePrefix, []uint{}, tx)
	})

	if err != nil {
		// 如果事务失败，重新加载策略以确保一致性
		_ = casbinx.GetEnforcer().LoadPolicy()
		return e.NewBusinessError(e.FAILURE, "删除用户失败")
	}
	return nil
}

// BindRole 绑定角色
func (s *AdminUserService) BindRole(params *form.BindRole) error {
	adminUserModel := model.NewAdminUsers()
	err := adminUserModel.GetById(adminUserModel, params.Id)
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "用户不存在")
	}

	// 检查角色是否存在
	// 判断是否有角色关联权限, 如果有则判断接口是否存在，只保留存在的接口ID
	ids, err := model.VerifyExistingIDs(model.NewRole(), params.RoleIds)
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "判断角色是否存在失败")
	}

	// 执行绑定角色事务
	err = model.DB().Transaction(func(tx *gorm.DB) error {
		return s.updateAdminUserRole(adminUserModel.ID, ids, tx)
	})

	if err != nil {
		// 如果事务失败，重新加载策略以确保一致性
		_ = casbinx.GetEnforcer().LoadPolicy()
		return e.NewBusinessError(e.FAILURE, "绑定角色失败")
	}

	return nil
}

// updateMenuPermissions 更新用户角色到关联中间表
func (s *AdminUserService) updateAdminUserRole(uid uint, roleIds []uint, tx ...*gorm.DB) error {
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
		if err := adminUserRoleMap.DeleteWithCondition(
			adminUserRoleMap,
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
		if err := adminUserRoleMap.BatchCreate(newMappings); err != nil {
			return err
		}
	}

	// 更新Casbin策略（如果 remainingList 为空，会自动删除所有策略）
	return s.editAdminUserPolicyRoles(uid, global.CasbinRolePrefix, remainingList, tx...)
}

// editAdminUserPolicyRoles 编辑用户的策略角色
func (s *AdminUserService) editAdminUserPolicyRoles(uid uint, childPrefix string, childIds []uint, tx ...*gorm.DB) error {
	enforcer := casbinx.GetEnforcer()
	if enforcer.Error() != nil {
		return e.NewBusinessError(1, "编辑失败")
	}
	// 设置事务，如果没有传入事务则清理之前的事务状态
	if len(tx) > 0 {
		enforcer.SetDB(tx[0])
	} else {
		enforcer.SetDB(nil)
	}
	userName := fmt.Sprintf("%s%s%d", global.CasbinAdminUserPrefix, global.CasbinSeparator, uid)
	// 如果 childIds 为空，直接删除所有策略
	if len(childIds) == 0 {
		_, err := enforcer.Enforcer.DeleteRolesForUser(userName)
		if err != nil {
			return err
		}
		return nil
	}
	policy := lo.Map(childIds, func(id uint, _ int) string {
		return fmt.Sprintf("%s:%d", childPrefix, id)
	})
	err := enforcer.EditPolicyRoles(userName, policy)
	if err != nil {
		return e.NewBusinessError(1, "编辑失败~")
	}
	return nil
}

// getImplicitRolesForAdminUser 获取用户的所有角色
func (s *AdminUserService) getImplicitRolesForAdminUser(userId uint) ([]string, error) {
	enforcer := casbinx.GetEnforcer()
	if enforcer.Error() != nil {
		return nil, e.NewBusinessError(1, "获取失败")
	}
	userName := fmt.Sprintf("%s%s%d", global.CasbinAdminUserPrefix, global.CasbinSeparator, userId)
	permissions, err := enforcer.GetImplicitRolesForUser(userName)
	if err != nil {
		return nil, e.NewBusinessError(1, "获取失败~")
	}
	return permissions, nil
}
