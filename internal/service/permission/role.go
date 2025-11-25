package permission

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	casbinx "github.com/wannanbigpig/gin-layout/internal/pkg/utils/casbin"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

const (
	maxRoleLevel      = 2 // 最多2层（即3级：顶级、二级、三级）
	maxChildrenPerTop = 5 // 每个顶级角色下最多5个子角色
)

// RoleService 角色服务
type RoleService struct {
	service.Base
}

// NewRoleService 创建角色服务实例
func NewRoleService() *RoleService {
	return &RoleService{}
}

// List 分页查询角色列表
func (s *RoleService) List(params *form.RoleList) interface{} {
	condition, args := s.buildListCondition(params)

	roleModel := model.NewRole()
	total, collection := model.ListPage(
		roleModel,
		params.Page,
		params.PerPage,
		condition,
		args,
		model.ListOptionalParams{
			OrderBy: "sort desc, id desc",
		},
	)

	return resources.ToRawCollection(params.Page, params.PerPage, total, collection)
}

// buildListCondition 构建列表查询条件
func (s *RoleService) buildListCondition(params *form.RoleList) (string, []any) {
	var conditions []string
	var args []any

	if params.Name != "" {
		conditions = append(conditions, "name like ?")
		args = append(args, "%"+params.Name+"%")
	}

	if params.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, params.Status)
	}

	// 父级ID过滤
	if params.Pid != nil {
		conditions = append(conditions, "pid = ?")
		args = append(args, params.Pid)
	}

	return strings.Join(conditions, " AND "), args
}

// Edit 编辑角色（新增或更新）
func (s *RoleService) Edit(params *form.EditRole) error {
	role := model.NewRole()
	editContext, err := s.prepareEditContext(role, params)
	if err != nil {
		return err
	}

	// 处理父级变化
	if err := s.handleParentChange(role, params); err != nil {
		return err
	}

	// 验证层级限制
	if role.Level > maxRoleLevel {
		return e.NewBusinessError(1, "最多只能创建2层角色")
	}

	// 赋值角色字段
	s.assignRoleFields(role, params)

	// 验证菜单ID有效性
	menuList, err := model.VerifyExistingIDs(model.NewMenu(), params.MenuList)
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "判断菜单是否存在失败")
	}

	// 执行编辑事务
	return s.executeEditTransaction(role, menuList, editContext.originPids, editContext.originPid)
}

// roleEditContext 角色编辑上下文信息
type roleEditContext struct {
	originPids string
	originPid  uint
}

// prepareEditContext 准备编辑上下文
func (s *RoleService) prepareEditContext(role *model.Role, params *form.EditRole) (*roleEditContext, error) {
	ctx := &roleEditContext{originPids: "0", originPid: 0}

	if params.Id > 0 {
		if err := role.GetById(role, params.Id); err != nil || role.ID == 0 {
			return nil, e.NewBusinessError(1, "编辑的角色不存在")
		}
		ctx.originPids = role.Pids
		ctx.originPid = role.Pid
	}

	return ctx, nil
}

// handleParentChange 处理父级变化
func (s *RoleService) handleParentChange(role *model.Role, params *form.EditRole) error {
	if params.Pid > 0 && params.Pid != role.Pid {
		return s.updateRoleWithParent(role, params)
	}

	if params.Pid == 0 {
		s.setRootRoleFields(role)
	}

	role.Pid = params.Pid
	return nil
}

// updateRoleWithParent 更新有父级的角色信息
func (s *RoleService) updateRoleWithParent(role *model.Role, params *form.EditRole) error {
	var parentRole model.Role
	if err := parentRole.GetById(&parentRole, params.Pid); err != nil || parentRole.ID == 0 {
		return e.NewBusinessError(1, "上级角色不存在")
	}

	// 防止循环引用
	if role.ID > 0 && utils2.WouldCauseCycle(role.ID, params.Pid, parentRole.Pids) {
		return e.NewBusinessError(1, "上级角色不能是当前角色自身或其子角色")
	}

	// 检查顶级角色下的子角色数量限制
	if parentRole.Pid == 0 {
		// 父角色是顶级角色，使用 children_num 字段检查其子角色数量
		// 如果是编辑操作且父级未变化，不需要检查（因为当前角色已经计算在内）
		// 如果是新增或父级变化，需要检查是否超过限制
		if role.ID == 0 || role.Pid != params.Pid {
			if parentRole.ChildrenNum >= maxChildrenPerTop {
				return e.NewBusinessError(1, fmt.Sprintf("每个顶级角色下最多只能创建%d个子角色", maxChildrenPerTop))
			}
		}
	}

	role.Level = parentRole.Level + 1
	role.Pids = s.buildPids(parentRole.Pids, parentRole.ID)
	role.Pid = params.Pid

	return nil
}

// setRootRoleFields 设置根角色字段
func (s *RoleService) setRootRoleFields(role *model.Role) {
	role.Level = 1
	role.Pids = "0"
	role.Pid = 0
}

// buildPids 构建父级ID序列
func (s *RoleService) buildPids(parentPids string, parentID uint) string {
	if parentPids == "0" || parentPids == "" {
		return fmt.Sprintf("%d", parentID)
	}
	return fmt.Sprintf("%s,%d", parentPids, parentID)
}

// assignRoleFields 赋值角色字段
func (s *RoleService) assignRoleFields(role *model.Role, params *form.EditRole) {
	role.Name = params.Name
	role.Description = params.Description
	role.Status = params.Status
	role.Sort = params.Sort
	// 注意：role.Pid 已在 handleParentChange 中设置
}

// executeEditTransaction 执行编辑事务
func (s *RoleService) executeEditTransaction(role *model.Role, menuList []uint, originPids string, originPid uint) error {
	err := role.DB().Transaction(func(tx *gorm.DB) error {
		// 保存角色信息
		if err := tx.Save(role).Error; err != nil {
			return err
		}

		// 更新子角色层级
		if err := s.updateChildrenLevels(role, originPids, tx); err != nil {
			return err
		}

		// 更新父级的 children_num
		// 如果 pid 发生变化，需要更新旧父级和新父级
		// 如果是新增操作（originPid = 0），只需要更新新父级
		if originPid > 0 && originPid != role.Pid {
			// 更新旧父级的 children_num（编辑操作且父级发生变化时）
			if err := model.UpdateChildrenNum(model.NewRole(), originPid, tx); err != nil {
				return err
			}
		}
		// 更新新父级的 children_num（新增或父级变化时都需要更新）
		if role.Pid > 0 && role.Pid != originPid {
			if err := model.UpdateChildrenNum(model.NewRole(), role.Pid, tx); err != nil {
				return err
			}
		}

		// 先更新角色菜单关联（因为 updateRoleMenu 会删除所有策略，包括继承关系）
		if err := s.updateRoleMenu(role.ID, menuList, tx); err != nil {
			return err
		}

		// 最后更新角色继承关系（在菜单关系之后，避免被删除）
		if err := s.updateRoleInheritance(role.ID, role.Pid, tx); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		// 如果事务失败，重新加载策略以确保一致性
		_ = casbinx.GetEnforcer().LoadPolicy()
		return err
	}

	return nil
}

// updateChildrenLevels 更新子角色层级
func (s *RoleService) updateChildrenLevels(role *model.Role, originPids string, tx *gorm.DB) error {
	if role.Pids == originPids {
		return nil
	}

	// 构建更新表达式
	updateExpr := s.buildPidsUpdateExpr(originPids, role.Pids)

	// 获取所有子角色ID
	var childRoleIds []uint
	if err := tx.Model(model.NewRole()).
		Where("FIND_IN_SET(?,pids)", role.ID).
		Pluck("id", &childRoleIds).Error; err != nil {
		return err
	}

	// 更新子角色的 pids 和 level
	if err := tx.Model(model.NewRole()).
		Where("FIND_IN_SET(?,pids)", role.ID).
		Updates(map[string]interface{}{
			"pids":  gorm.Expr(updateExpr),
			"level": gorm.Expr("length(pids) - length(replace(pids, ',', '')) + 1"),
		}).Error; err != nil {
		return err
	}

	// 批量更新所有子角色的继承关系
	if len(childRoleIds) > 0 {
		if err := s.batchUpdateRoleInheritances(childRoleIds, role.ID, tx); err != nil {
			return err
		}
	}

	return nil
}

// batchUpdateRoleInheritances 批量更新角色继承关系
func (s *RoleService) batchUpdateRoleInheritances(childRoleIds []uint, parentRoleId uint, tx *gorm.DB) error {
	enforcer := casbinx.GetEnforcer()
	if enforcer == nil || enforcer.Error() != nil {
		return e.NewBusinessError(1, "casbin 初始化失败")
	}

	enforcer.SetDB(tx)
	parentRoleName := fmt.Sprintf("%s%s%d", global.CasbinRolePrefix, global.CasbinSeparator, parentRoleId)

	return enforcer.WithTransaction(func(e casbin.IEnforcer) error {
		// 批量删除所有子角色的角色继承关系
		var allPoliciesToRemove [][]string
		for _, childId := range childRoleIds {
			childRoleName := fmt.Sprintf("%s%s%d", global.CasbinRolePrefix, global.CasbinSeparator, childId)
			roles, err := e.GetRolesForUser(childRoleName)
			if err != nil {
				return err
			}

			rolePrefix := global.CasbinRolePrefix + global.CasbinSeparator
			for _, role := range roles {
				if strings.HasPrefix(role, rolePrefix) {
					allPoliciesToRemove = append(allPoliciesToRemove, []string{childRoleName, role})
				}
			}
		}

		// 批量删除
		if len(allPoliciesToRemove) > 0 {
			if _, err := e.RemoveGroupingPolicies(allPoliciesToRemove); err != nil {
				return err
			}
		}

		// 批量添加新的继承关系
		if parentRoleId > 0 {
			var policiesToAdd [][]string
			for _, childId := range childRoleIds {
				childRoleName := fmt.Sprintf("%s%s%d", global.CasbinRolePrefix, global.CasbinSeparator, childId)
				policiesToAdd = append(policiesToAdd, []string{childRoleName, parentRoleName})
			}

			if len(policiesToAdd) > 0 {
				ok, err := e.AddGroupingPolicies(policiesToAdd)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("批量添加角色继承关系失败")
				}
			}
		}

		return nil
	})
}

// buildPidsUpdateExpr 构建pids更新表达式
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

// updateRoleMenu 更新角色菜单关联
func (s *RoleService) updateRoleMenu(roleId uint, menuList []uint, tx ...*gorm.DB) error {
	roleMenuMap := model.NewRoleMenuMap()
	if len(tx) > 0 {
		roleMenuMap.SetDB(tx[0])
	}

	// 保存用户实际选择的菜单到数据库（不包含自动添加的父级目录）
	// 这样回显时只显示用户实际选择的菜单

	// 获取现有关联
	existingIds, err := model.ExtractColumnsByCondition[model.RoleMenuMap, *model.RoleMenuMap, uint](
		roleMenuMap,
		"menu_id",
		"role_id = ?",
		roleId,
	)
	if err != nil {
		return err
	}

	// 计算差集（基于用户实际选择的菜单）
	toDelete, toAdd, _ := utils.CalculateChanges(existingIds, menuList)

	// 删除不再需要的关联
	if len(toDelete) > 0 {
		if err := roleMenuMap.DeleteWithCondition(
			roleMenuMap,
			"role_id = ? AND menu_id IN (?)",
			[]any{roleId, toDelete}...,
		); err != nil {
			return err
		}
	}

	// 批量创建新关联（只保存用户实际选择的菜单）
	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(menuId uint, _ int) *model.RoleMenuMap {
			return &model.RoleMenuMap{
				RoleId: roleId,
				MenuId: menuId,
			}
		})
		if err := roleMenuMap.BatchCreate(newMappings); err != nil {
			return err
		}
	}

	// 更新 Casbin 策略时，自动包含所有父级目录，确保权限检查时菜单树完整
	// 这样在 GetUserMenuInfo 中获取权限时，能正确显示所有父级目录
	casbinMenuList := s.includeParentMenus(menuList, tx...)
	// 如果 casbinMenuList 为空，会自动删除所有策略
	return s.editRolePolicyRoles(roleId, global.CasbinMenuPrefix, casbinMenuList, tx...)
}

// editRolePolicyRoles 编辑角色的策略角色
func (s *RoleService) editRolePolicyRoles(roleId uint, childPrefix string, childIds []uint, tx ...*gorm.DB) error {
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
	roleName := fmt.Sprintf("%s%s%d", global.CasbinRolePrefix, global.CasbinSeparator, roleId)
	// 如果 childIds 为空，直接删除所有策略
	if len(childIds) == 0 {
		_, err := enforcer.Enforcer.DeleteRolesForUser(roleName)
		if err != nil {
			return err
		}
		return nil
	}
	policy := lo.Map(childIds, func(id uint, _ int) string {
		return fmt.Sprintf("%s:%d", childPrefix, id)
	})
	err := enforcer.EditPolicyRoles(roleName, policy)
	if err != nil {
		return e.NewBusinessError(1, "编辑失败~")
	}
	return nil
}

// includeParentMenus 自动包含菜单的所有父级目录
// 在角色绑定菜单时，自动将父级目录也一并绑定，确保菜单树完整性
func (s *RoleService) includeParentMenus(menuIDs []uint, tx ...*gorm.DB) []uint {
	if len(menuIDs) == 0 {
		return menuIDs
	}

	menuModel := model.NewMenu()
	if len(tx) > 0 {
		menuModel.SetDB(tx[0])
	}

	// 查询这些菜单的 pids 字段
	menus := model.List(menuModel, "id IN ?", []any{menuIDs}, model.ListOptionalParams{
		SelectFields: []string{"id", "pids"},
	})

	// 使用 map 去重，包含原始菜单ID和所有父级菜单ID
	allMenuIDSet := make(map[uint]struct{})

	// 先添加原始菜单ID
	for _, menuID := range menuIDs {
		allMenuIDSet[menuID] = struct{}{}
	}

	// 解析每个菜单的 pids 字段，提取所有父级菜单ID
	for _, menu := range menus {
		if menu.Pids == "" || menu.Pids == "0" {
			continue
		}

		// 解析 pids 字符串，格式如 "0,1,2"
		pids := strings.Split(menu.Pids, ",")
		for _, pidStr := range pids {
			pidStr = strings.TrimSpace(pidStr)
			if pidStr == "" || pidStr == "0" {
				continue
			}
			if pid, err := strconv.ParseUint(pidStr, 10, 64); err == nil {
				allMenuIDSet[uint(pid)] = struct{}{}
			}
		}
	}

	// 转换为切片
	allMenuIDs := make([]uint, 0, len(allMenuIDSet))
	for menuID := range allMenuIDSet {
		allMenuIDs = append(allMenuIDs, menuID)
	}

	return allMenuIDs
}

// updateRoleInheritance 更新角色继承关系
func (s *RoleService) updateRoleInheritance(childRoleId uint, parentRoleId uint, tx ...*gorm.DB) error {
	enforcer := casbinx.GetEnforcer()
	if enforcer == nil || enforcer.Error() != nil {
		return e.NewBusinessError(1, "casbin 初始化失败")
	}

	if len(tx) > 0 {
		enforcer.SetDB(tx[0])
	}

	childRoleName := fmt.Sprintf("%s%s%d", global.CasbinRolePrefix, global.CasbinSeparator, childRoleId)

	return enforcer.WithTransaction(func(e casbin.IEnforcer) error {
		// 删除所有角色继承关系
		if err := s.removeAllRoleInheritances(e, childRoleName); err != nil {
			return err
		}

		// 如果有父角色，建立继承关系
		if parentRoleId > 0 {
			parentRoleName := fmt.Sprintf("%s%s%d", global.CasbinRolePrefix, global.CasbinSeparator, parentRoleId)
			ok, err := e.AddGroupingPolicy(childRoleName, parentRoleName)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("添加角色继承关系失败")
			}
		}

		return nil
	})
}

// removeAllRoleInheritances 删除所有角色继承关系
func (s *RoleService) removeAllRoleInheritances(e casbin.IEnforcer, childRoleName string) error {
	roles, err := e.GetRolesForUser(childRoleName)
	if err != nil {
		return err
	}

	rolePrefix := global.CasbinRolePrefix + global.CasbinSeparator
	// 收集所有需要删除的角色继承关系
	var policiesToRemove [][]string
	for _, role := range roles {
		if strings.HasPrefix(role, rolePrefix) {
			policiesToRemove = append(policiesToRemove, []string{childRoleName, role})
		}
	}

	// 批量删除（如果没有需要删除的策略，直接返回）
	if len(policiesToRemove) == 0 {
		return nil
	}

	// 使用批量删除方法
	_, err = e.RemoveGroupingPolicies(policiesToRemove)
	return err
}

// Delete 删除角色
func (s *RoleService) Delete(id uint) error {
	role := model.NewRole()
	if err := role.GetById(role, id); err != nil || role.ID == 0 {
		return e.NewBusinessError(1, "角色不存在")
	}

	// 检查是否有子角色（使用 children_num 字段判断，性能更好）
	if role.ChildrenNum > 0 {
		return e.NewBusinessError(1, "该角色有子角色，无法删除")
	}

	// 执行删除事务
	return s.executeDeleteTransaction(role, id)
}

// executeDeleteTransaction 执行删除事务
func (s *RoleService) executeDeleteTransaction(role *model.Role, id uint) error {
	err := role.DB().Transaction(func(tx *gorm.DB) error {
		// 删除角色菜单关联
		roleMenuMap := model.NewRoleMenuMap()
		roleMenuMap.SetDB(tx)
		if err := roleMenuMap.DeleteWithCondition(roleMenuMap, "role_id = ?", id); err != nil {
			return err
		}

		// 保存父级ID，用于后续更新 children_num
		parentId := role.Pid

		// 删除角色
		if err := tx.Delete(role, id).Error; err != nil {
			return err
		}

		// 更新父级的 children_num
		if parentId > 0 {
			if err := model.UpdateChildrenNum(model.NewRole(), parentId, tx); err != nil {
				return err
			}
		}

		// 删除Casbin策略
		return s.deleteCasbinPolicyForRole(id, tx)
	})

	if err != nil {
		// 如果事务失败，重新加载策略以确保一致性
		_ = casbinx.GetEnforcer().LoadPolicy()
		return e.NewBusinessError(1, "删除角色失败")
	}

	return nil
}

// deleteCasbinPolicyForRole 删除角色的Casbin策略
func (s *RoleService) deleteCasbinPolicyForRole(roleId uint, tx *gorm.DB) error {
	return s.deleteAllPoliciesForRole(roleId, tx)
}

// deleteAllPoliciesForRole 删除角色的所有Casbin策略
// 包括：
// 1. 角色作为 user 的所有策略（角色关联的菜单和角色继承关系）
// 2. 所有引用该角色的策略（用户和部门引用该角色的策略）
func (s *RoleService) deleteAllPoliciesForRole(roleId uint, tx *gorm.DB) error {
	enforcer := casbinx.GetEnforcer()
	if enforcer == nil || enforcer.Error() != nil {
		return nil
	}

	enforcer.SetDB(tx)
	roleName := fmt.Sprintf("%s%s%d", global.CasbinRolePrefix, global.CasbinSeparator, roleId)

	return enforcer.WithTransaction(func(e casbin.IEnforcer) error {
		// 删除角色作为 user 的所有策略（角色关联的菜单和角色继承关系）
		_, _ = e.DeleteRolesForUser(roleName)

		// 删除所有引用该角色的策略（用户和部门引用该角色的策略）
		// 使用 RemoveFilteredGroupingPolicy 删除所有第二个参数匹配的策略
		// [g, adminUser:*, role:roleId] 和 [g, dept:*, role:roleId]
		_, _ = e.RemoveFilteredGroupingPolicy(1, roleName)

		return nil
	})
}

// Detail 获取角色详情
func (s *RoleService) Detail(id uint) (any, error) {
	role := model.NewRole()
	if err := role.GetAllById(role, id); err != nil || role.ID == 0 {
		return nil, e.NewBusinessError(1, "角色不存在")
	}
	return resources.NewRoleTransformer().ToStruct(role), nil
}

// GetRoleMenus 获取角色的所有菜单（包括直接绑定的菜单和继承自父角色的菜单）
func (s *RoleService) GetRoleMenus(roleId uint) ([]string, error) {
	role := model.NewRole()
	if err := role.GetById(role, roleId); err != nil || role.ID == 0 {
		return nil, e.NewBusinessError(1, "角色不存在")
	}
	return s.getImplicitRolesForRole(roleId)
}

// getImplicitRolesForRole 获取角色的所有角色（包括直接绑定的菜单和继承自父角色的菜单）
func (s *RoleService) getImplicitRolesForRole(roleId uint) ([]string, error) {
	enforcer := casbinx.GetEnforcer()
	if enforcer.Error() != nil {
		return nil, e.NewBusinessError(1, "获取失败")
	}
	roleName := fmt.Sprintf("%s%s%d", global.CasbinRolePrefix, global.CasbinSeparator, roleId)
	permissions, err := enforcer.GetImplicitRolesForUser(roleName)
	if err != nil {
		return nil, e.NewBusinessError(1, "获取失败~")
	}
	return permissions, nil
}
