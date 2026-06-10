package role

import (
	"context"
	"errors"
	"strings"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
)

// roleMutation 角色变更参数，用于封装新增/更新角色的请求数据。
type roleMutation struct {
	Id          uint   // 角色 ID，0 表示新增
	Code        string // 角色编码，新增时为空则自动生成
	Name        string // 角色名称
	Description string // 角色描述
	Status      uint8  // 角色状态
	Sort        uint   // 排序权重
	MenuList    []uint // 关联的菜单 ID 列表
	DataScope   uint8  // 数据权限范围:1全部,2本部门及子级,3本部门,4仅本人,5自定义部门
	DeptIds     []uint // 自定义部门 ID 列表（仅 DataScope=5 时有效）
}

// applyRoleMutation 执行角色变更操作（新增/更新）。
// 处理逻辑：
// 1. 验证角色是否存在（更新时）
// 2. 检查受保护角色（系统默认角色不可修改）
// 3. 填充角色基础字段
// 4. 验证菜单列表
// 5. 事务保存：角色数据、菜单关联、同步受影响用户权限
// 6. 同步受影响角色的用户权限缓存
func (s *RoleService) applyRoleMutation(params *roleMutation) error {
	role := model.NewRole()
	if params.Id > 0 {
		if err := role.GetById(params.Id); err != nil || role.ID == 0 {
			return e.NewBusinessError(e.RoleNotFound)
		}
	}
	// 检查是否为受保护角色（系统默认角色不可修改）
	if params.Id > 0 && access.NewSystemDefaultsService().IsProtectedRole(role) {
		return e.NewBusinessError(e.SuperAdminCannotModify)
	}

	trimmedCode := strings.TrimSpace(params.Code)
	if params.Id == 0 && trimmedCode != "" {
		existing := model.NewRole()
		if err := existing.FindByCode(trimmedCode); err == nil && existing.ID > 0 {
			return e.NewBusinessError(e.RoleExists)
		}
	}

	// 新增角色时生成 code
	if params.Id == 0 {
		if trimmedCode != "" {
			role.Code = trimmedCode
		} else if role.Code == "" {
			role.Code = s.generateRoleCode()
		}
	}
	// 填充可变更字段
	role.Name = params.Name
	role.Description = params.Description
	role.Status = params.Status
	role.Sort = params.Sort
	role.DataScope = normalizeDataScope(params.DataScope)

	// 验证所有菜单 ID 是否存在，再按真实授权节点规则过滤。
	menuList, err := model.VerifyExistingIDs(model.NewMenu(), params.MenuList)
	if err != nil {
		return e.NewBusinessError(e.MenuNotFound)
	}
	menuList, err = access.NewRoleMenuAssignmentService().NormalizeMenuIDs(menuList)
	if err != nil {
		return err
	}

	db, err := role.GetDB()
	if err != nil {
		return err
	}
	// 事务执行：保存角色、级联更新、菜单同步、受影响用户权限同步；
	// 提交后的共享策略缓存刷新失败仅告警，不再向调用方回传“失败但数据已落库”的假象。
	coordinator := access.NewPermissionSyncCoordinator()
	var affectedUserIDs []uint
	if err := coordinator.RunAfterCommitWithCode(db, e.FAILURE, func(tx *gorm.DB) error {
		role.SetDB(tx)

		if err := role.Save(); err != nil {
			if isRoleCodeDuplicateErr(err) {
				return e.NewBusinessError(e.RoleExists)
			}
			return err
		}

		if err := s.updateRoleMenu(role.ID, menuList, tx); err != nil {
			return err
		}
		// 更新角色自定义数据权限部门关联
		if err := s.updateRoleDataScopeDept(role.ID, params.DataScope, params.DeptIds, tx); err != nil {
			return err
		}
		// 同步受影响角色的用户权限（纳入事务，与提交一致）
		affected, err := access.NewAffectedUsersResolver().Resolve(access.PermissionChangeScope{RoleIDs: []uint{role.ID}}, tx)
		if err != nil {
			return err
		}
		affectedUserIDs = affected
		return coordinator.SyncUsers(affectedUserIDs, tx)
	}); err != nil {
		return err
	}
	access.NewDataScopeFilter().InvalidateCache(context.Background(), affectedUserIDs...)
	return nil
}

func normalizeDataScope(dataScope uint8) uint8 {
	if dataScope == 0 {
		return access.DataScopeDept
	}
	return dataScope
}

func isRoleCodeDuplicateErr(err error) bool {
	if err == nil {
		return false
	}

	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062 && strings.Contains(mysqlErr.Message, "role.uniq_code_deleted_at")
	}

	return strings.Contains(err.Error(), "Duplicate entry") && strings.Contains(err.Error(), "role.uniq_code_deleted_at")
}

// generateRoleCode 生成角色唯一编码，格式：role_{uuid}。
func (s *RoleService) generateRoleCode() string {
	return "role_" + uuid.NewString()
}

// updateRoleMenu 更新角色的菜单关联关系。
// 使用差分算法：计算需要删除和新增的菜单 ID，只变更差异部分。
// 参数：
//   - roleId: 角色 ID
//   - menuList: 目标菜单 ID 列表
//   - tx: 可选的事务 DB 实例
func (s *RoleService) updateRoleMenu(roleId uint, menuList []uint, tx ...*gorm.DB) error {
	roleMenuMap := model.NewRoleMenuMap()
	if len(tx) > 0 {
		roleMenuMap.SetDB(tx[0])
	}

	db, err := roleMenuMap.GetDB(roleMenuMap)
	if err != nil {
		return err
	}
	// 查询角色当前已关联的菜单 ID 列表
	var existingIds []uint
	if err := db.Where("role_id = ?", roleId).Pluck("menu_id", &existingIds).Error; err != nil {
		return err
	}

	// 计算差异：toDelete 需删除，toAdd 需新增
	toDelete, toAdd, _ := utils.CalculateChanges(existingIds, menuList)
	// 批量删除差异菜单关联
	if len(toDelete) > 0 {
		if err := roleMenuMap.DeleteWhere("role_id = ? AND menu_id IN (?)", []any{roleId, toDelete}...); err != nil {
			return err
		}
	}

	// 批量新增菜单关联
	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(menuId uint, _ int) *model.RoleMenuMap {
			return &model.RoleMenuMap{RoleId: roleId, MenuId: menuId}
		})
		if err := roleMenuMap.CreateBatch(newMappings); err != nil {
			return err
		}
	}
	return nil
}

// executeDeleteTransaction 执行角色删除事务。
// 处理逻辑：
// 1. 删除角色 - 菜单关联
// 2. 删除角色记录
// 3. 同步受影响用户的权限缓存
func (s *RoleService) executeDeleteTransaction(role *model.Role, id uint) error {
	db, err := role.GetDB()
	if err != nil {
		return e.NewBusinessError(e.RoleCannotDelete)
	}
	affectedUserIDs, err := access.NewAffectedUsersResolver().Resolve(access.PermissionChangeScope{RoleIDs: []uint{id}})
	if err != nil {
		return e.NewBusinessError(e.RoleCannotDelete)
	}
	coordinator := access.NewPermissionSyncCoordinator()
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		role.SetDB(tx)

		// 删除角色关联的所有菜单
		roleMenuMap := model.NewRoleMenuMap()
		roleMenuMap.SetDB(tx)
		if err := roleMenuMap.DeleteWhere("role_id = ?", id); err != nil {
			return err
		}
		adminUserRoleMap := model.NewAdminUserRoleMap()
		adminUserRoleMap.SetDB(tx)
		if err := adminUserRoleMap.DeleteWhere("role_id = ?", id); err != nil {
			return err
		}
		deptRoleMap := model.NewDeptRoleMap()
		deptRoleMap.SetDB(tx)
		if err := deptRoleMap.DeleteWhere("role_id = ?", id); err != nil {
			return err
		}
		// 删除角色自定义数据权限部门关联
		roleDataScopeDept := model.NewRoleDataScopeDept()
		roleDataScopeDept.SetDB(tx)
		if err := roleDataScopeDept.DeleteByRoleId(id); err != nil {
			return err
		}

		// 删除角色记录
		if _, err := role.DeleteByID(id); err != nil {
			return err
		}
		return coordinator.SyncUsers(affectedUserIDs, tx)
	})
	if err != nil {
		return e.NewBusinessError(e.RoleCannotDelete)
	}
	// 同步受影响用户的权限缓存
	if err := coordinator.ReloadPolicyCacheWithCode(e.RoleCannotDelete); err != nil {
		return err
	}
	access.NewDataScopeFilter().InvalidateCache(context.Background(), affectedUserIDs...)
	return nil
}

// updateRoleDataScopeDept 更新角色的自定义数据权限部门关联。
// 使用差分算法：计算需要删除和新增的部门 ID，只变更差异部分。
// 仅当 dataScope=DataScopeCustom（自定义部门）时才保存关联。
func (s *RoleService) updateRoleDataScopeDept(roleId uint, dataScope uint8, deptIds []uint, tx ...*gorm.DB) error {
	roleDataScopeDept := model.NewRoleDataScopeDept()
	if len(tx) > 0 {
		roleDataScopeDept.SetDB(tx[0])
	}

	db, err := roleDataScopeDept.GetDB(roleDataScopeDept)
	if err != nil {
		return err
	}
	// 查询角色当前已关联的部门 ID 列表
	var existingIds []uint
	if err := db.Where("role_id = ?", roleId).Pluck("dept_id", &existingIds).Error; err != nil {
		return err
	}

	// 非自定义部门模式时，清空关联
	if dataScope != access.DataScopeCustom {
		deptIds = nil
	} else {
		deptIds, err = existingDepartmentIDs(db, deptIds)
		if err != nil {
			return err
		}
	}

	// 计算差异：toDelete 需删除，toAdd 需新增
	toDelete, toAdd, _ := utils.CalculateChanges(existingIds, deptIds)
	// 批量删除差异部门关联
	if len(toDelete) > 0 {
		if err := roleDataScopeDept.DeleteWhere("role_id = ? AND dept_id IN (?)", []any{roleId, toDelete}...); err != nil {
			return err
		}
	}

	// 批量新增部门关联
	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(deptId uint, _ int) *model.RoleDataScopeDept {
			return &model.RoleDataScopeDept{RoleId: roleId, DeptId: deptId}
		})
		if err := roleDataScopeDept.CreateBatch(newMappings); err != nil {
			return err
		}
	}
	return nil
}

func existingDepartmentIDs(db *gorm.DB, deptIds []uint) ([]uint, error) {
	if len(deptIds) == 0 {
		return deptIds, nil
	}
	if db == nil {
		var err error
		db, err = model.GetDB(model.NewDepartment())
		if err != nil {
			return nil, err
		}
	}
	var existingIds []uint
	if err := db.Session(&gorm.Session{NewDB: true}).Model(&model.Department{}).Where("id IN ?", deptIds).Pluck("id", &existingIds).Error; err != nil {
		return nil, err
	}
	return existingIds, nil
}
