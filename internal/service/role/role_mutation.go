package role

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

// roleMutation 角色变更参数，用于封装新增/更新角色的请求数据。
type roleMutation struct {
	Id          uint   // 角色 ID，0 表示新增
	Code        string // 角色编码，新增时为空则自动生成
	Name        string // 角色名称
	Description string // 角色描述
	Status      uint8  // 角色状态
	Pid         uint   // 父角色 ID，0 表示顶级角色
	Sort        uint   // 排序权重
	MenuList    []uint // 关联的菜单 ID 列表
}

// applyRoleMutation 执行角色变更操作（新增/更新）。
// 处理逻辑：
// 1. 验证角色是否存在（更新时）
// 2. 检查受保护角色（系统默认角色不可修改）
// 3. 验证并构建树形路径（pids, level）
// 4. 填充角色基础字段
// 5. 验证菜单列表
// 6. 事务保存：角色数据、级联更新子角色 pids、更新子角色数量、同步菜单关联
// 7. 同步受影响角色的用户权限缓存
func (s *RoleService) applyRoleMutation(params *roleMutation) error {
	role := model.NewRole()
	originPids := "0"
	originPid := uint(0)
	// 更新场景：加载现有角色数据，记录原始 pids 用于后续级联判断
	if params.Id > 0 {
		if err := role.GetById(params.Id); err != nil || role.ID == 0 {
			return e.NewBusinessError(e.RoleNotFound)
		}
		originPids = role.Pids
		originPid = role.Pid
	}
	// 检查是否为受保护角色（系统默认角色不可修改）
	if params.Id > 0 && access.NewSystemDefaultsService().IsProtectedRole(role) {
		return e.NewBusinessError(e.SuperAdminCannotModify)
	}

	// 处理父角色变更：验证父角色、检测环路、计算层级和路径
	if params.Pid > 0 && params.Pid != role.Pid {
		parentRole := model.NewRole()
		if err := parentRole.GetById(params.Pid); err != nil || parentRole.ID == 0 {
			return e.NewBusinessError(e.ParentRoleNotExists)
		}

		// 环路检测：当前角色若已在父角色的祖先路径上，选择该父角色会形成环
		if role.ID > 0 && utils2.WouldCauseCycle(role.ID, params.Pid, parentRole.Pids) {
			return e.NewBusinessError(e.ParentRoleInvalid)
		}

		// 限制顶级角色的子角色数量
		if parentRole.Pid == 0 && (role.ID == 0 || role.Pid != params.Pid) && parentRole.ChildrenNum >= maxChildrenPerTop {
			return e.NewBusinessError(e.MaxChildRoles)
		}

		// 构建新的层级和路径：父层级 +1，pids = 父 pids + 父 ID
		role.Level = parentRole.Level + 1
		if parentRole.Pids == "0" || parentRole.Pids == "" {
			role.Pids = fmt.Sprintf("%d", parentRole.ID)
		} else {
			role.Pids = fmt.Sprintf("%s,%d", parentRole.Pids, parentRole.ID)
		}
		role.Pid = params.Pid
	} else if params.Pid == 0 {
		// 设置为顶级角色
		role.Level = 1
		role.Pids = "0"
		role.Pid = 0
	} else {
		// 父角色未变更，仅同步 pid 字段
		role.Pid = params.Pid
	}
	// 检查角色层级深度是否超限
	if role.Level > maxRoleLevel {
		return e.NewBusinessError(e.MaxRoleDepth)
	}

	// 新增角色时生成 code
	if params.Id == 0 {
		if params.Code != "" {
			role.Code = params.Code
		} else if role.Code == "" {
			role.Code = s.generateRoleCode()
		}
	}
	// 填充可变更字段
	role.Name = params.Name
	role.Description = params.Description
	role.Status = params.Status
	role.Sort = params.Sort

	// 验证所有菜单 ID 是否存在
	menuList, err := model.VerifyExistingIDs(model.NewMenu(), params.MenuList)
	if err != nil {
		return e.NewBusinessError(e.MenuNotFound)
	}

	db, err := role.GetDB()
	if err != nil {
		return err
	}
	// 事务执行：保存角色、级联更新、菜单同步
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		role.SetDB(tx)

		if err := role.Save(); err != nil {
			return err
		}
		// pids 变更时，级联更新所有子角色的 pids 路径
		if role.Pids != originPids {
			updateExpr := s.buildPidsUpdateExpr(originPids, role.Pids)
			roleModel := model.NewRole()
			roleModel.SetDB(tx)
			if err := roleModel.UpdateChildrenPidsByParent(role.ID, updateExpr); err != nil {
				return err
			}
		}

		// 原父角色的子角色数量减 1
		if originPid > 0 && originPid != role.Pid {
			if err := model.UpdateChildrenNum(model.NewRole(), originPid, tx); err != nil {
				return err
			}
		}
		// 新父角色的子角色数量加 1
		if role.Pid > 0 && role.Pid != originPid {
			if err := model.UpdateChildrenNum(model.NewRole(), role.Pid, tx); err != nil {
				return err
			}
		}

		return s.updateRoleMenu(role.ID, menuList, tx)
	})
	if err != nil {
		return err
	}
	// 同步受影响角色的用户权限缓存
	return access.NewPermissionSyncCoordinator().SyncUsersAffectedByRoles([]uint{role.ID})
}

// generateRoleCode 生成角色唯一编码，格式：role_{uuid}。
func (s *RoleService) generateRoleCode() string {
	return "role_" + uuid.NewString()
}

// buildPidsUpdateExpr 构建 SQL CASE 表达式，用于级联更新子角色的 pids 路径。
// 场景：当某角色的 pids 变更时，其所有子角色的 pids 前缀需要同步更新。
// 参数：
//   - originPids: 原始路径
//   - newPids: 新路径
//
// 返回：SQL CASE 表达式字符串
// 示例：originPids="1,2", newPids="1,8" 时，子角色 "1,2,3" → "1,8,3"
func (s *RoleService) buildPidsUpdateExpr(originPids, newPids string) string {
	if originPids == "0" {
		return fmt.Sprintf(
			"CASE WHEN pids = '0' THEN '%s' WHEN pids LIKE '0,%%' THEN CONCAT('%s,', SUBSTRING(pids, 3)) ELSE pids END",
			newPids, newPids,
		)
	}

	return fmt.Sprintf(
		"CASE WHEN pids = '%s' THEN '%s' WHEN pids LIKE '%s,%%' THEN CONCAT('%s,', SUBSTRING(pids, %d)) ELSE pids END",
		originPids, newPids, originPids, newPids, len(originPids)+2,
	)
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

	// 查询角色当前已关联的菜单 ID 列表
	existingIds, err := model.ExtractColumnsByCondition[model.RoleMenuMap, *model.RoleMenuMap, uint](roleMenuMap, "menu_id", "role_id = ?", roleId)
	if err != nil {
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
// 3. 更新原父角色的子角色数量
// 4. 同步受影响用户的权限缓存
func (s *RoleService) executeDeleteTransaction(role *model.Role, id uint) error {
	db, err := role.GetDB()
	if err != nil {
		return e.NewBusinessError(e.RoleCannotDelete)
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		role.SetDB(tx)

		// 删除角色关联的所有菜单
		roleMenuMap := model.NewRoleMenuMap()
		roleMenuMap.SetDB(tx)
		if err := roleMenuMap.DeleteWhere("role_id = ?", id); err != nil {
			return err
		}

		// 删除角色记录
		parentId := role.Pid
		if _, err := role.DeleteByID(id); err != nil {
			return err
		}
		// 更新原父角色的子角色数量（减 1）
		if parentId > 0 {
			if err := model.UpdateChildrenNum(model.NewRole(), parentId, tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return e.NewBusinessError(e.RoleCannotDelete)
	}
	// 同步受影响用户的权限缓存
	return access.NewPermissionSyncCoordinator().SyncUsersAffectedByRoles([]uint{id})
}
