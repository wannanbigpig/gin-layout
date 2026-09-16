package access

import (
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

// -------------------- 单用户权限展开链路 --------------------

// collectUserPolicies 根据数据库关系展开单个用户的最终接口权限。
func (s *UserPermissionSyncService) collectUserPolicies(userID uint, tx ...*gorm.DB) ([][]string, error) {
	userInfo := model.NewAdminUsers()
	if t := FirstTx(tx); t != nil {
		userInfo.SetDB(t)
	}
	if err := userInfo.GetById(userID); err != nil {
		return nil, err
	}
	if userInfo.ID == 0 || userInfo.Status != model.AdminUserStatusEnabled || userInfo.ID == global.SuperAdminId {
		return nil, nil
	}

	roleIDs, err := s.userRoleIDs(userID, tx...)
	if err != nil {
		return nil, err
	}

	menuIDs, err := s.RoleMenuIDs(roleIDs, tx...)
	if err != nil {
		return nil, err
	}

	if len(menuIDs) == 0 {
		return nil, nil
	}
	menuApiMapModel := model.NewMenuApiMap()
	if t := FirstTx(tx); t != nil {
		menuApiMapModel.SetDB(t)
	}
	permissions, err := menuApiMapModel.ApiPermissionsByMenuIds(menuIDs)
	if err != nil {
		return nil, err
	}
	policies := make([][]string, 0, len(permissions))
	for _, permission := range permissions {
		if permission.Route != "" && permission.Method != "" {
			policies = append(policies, []string{permission.Route, permission.Method})
		}
	}
	return policies, nil
}

// -------------------- 批量用户权限同步链路 --------------------

func (s *UserPermissionSyncService) collectPoliciesForUsers(userIDs []uint, tx *gorm.DB) (map[uint][][]string, error) {
	uniqueIDs := UniqueUintSlice(userIDs)
	result := make(map[uint][][]string, len(uniqueIDs))
	if len(uniqueIDs) == 0 {
		return result, nil
	}

	userModel := model.NewAdminUsers()
	userModel.SetDB(tx)
	users, err := userModel.SyncUserRows(uniqueIDs)
	if err != nil {
		return nil, err
	}
	activeUserIDs := make([]uint, 0, len(users))
	for _, user := range users {
		if user.Status != model.AdminUserStatusEnabled || user.ID == global.SuperAdminId || user.IsSuperAdmin == global.Yes {
			result[user.ID] = nil
			continue
		}
		activeUserIDs = append(activeUserIDs, user.ID)
	}
	if len(activeUserIDs) == 0 {
		return result, nil
	}

	userRoleMap := make(map[uint][]uint, len(activeUserIDs))
	roleMapModel := model.NewAdminUserRoleMap()
	roleMapModel.SetDB(tx)
	directRoles, err := roleMapModel.UserRoleMapByUids(activeUserIDs)
	if err != nil {
		return nil, err
	}
	for userID, roleIDs := range directRoles {
		userRoleMap[userID] = append(userRoleMap[userID], roleIDs...)
	}
	deptMapModel := model.NewAdminUserDeptMap()
	deptMapModel.SetDB(tx)
	userDepts, deptIDs, err := deptMapModel.UserDeptMapByUids(activeUserIDs)
	if err != nil {
		return nil, err
	}
	if len(deptIDs) > 0 {
		deptRoleMapModel := model.NewDeptRoleMap()
		deptRoleMapModel.SetDB(tx)
		deptRoleMap, err := deptRoleMapModel.DeptRoleMapByDeptIds(deptIDs)
		if err != nil {
			return nil, err
		}
		for userID, userDeptIDs := range userDepts {
			for _, deptID := range userDeptIDs {
				userRoleMap[userID] = append(userRoleMap[userID], deptRoleMap[deptID]...)
			}
			userRoleMap[userID] = UniqueUintSlice(userRoleMap[userID])
		}
	}
	if len(userRoleMap) == 0 {
		return result, nil
	}

	userExpandedRoles, allRoleIDs := expandUserRoles(userRoleMap)
	if len(allRoleIDs) == 0 {
		return result, nil
	}
	roleModel := model.NewRole()
	roleModel.SetDB(tx)
	enabledRoleIDs, err := roleModel.EnabledIdsByIds(allRoleIDs)
	if err != nil {
		return nil, err
	}
	enabledRoles := make(map[uint]struct{}, len(enabledRoleIDs))
	for _, roleID := range enabledRoleIDs {
		enabledRoles[roleID] = struct{}{}
	}
	userExpandedRoles, allRoleIDs = filterUserRolesByEnabledSet(userExpandedRoles, enabledRoles)
	if len(allRoleIDs) == 0 {
		return result, nil
	}

	roleMenuMapModel := model.NewRoleMenuMap()
	roleMenuMapModel.SetDB(tx)
	roleMenus, err := roleMenuMapModel.RoleMenuMapByRoleIds(allRoleIDs)
	if err != nil || len(roleMenus) == 0 {
		return result, err
	}
	roleMenuMap := roleMenuIDMap(roleMenus)

	menuIDs := make([]uint, 0)
	for _, roleMenuIDs := range roleMenuMap {
		menuIDs = append(menuIDs, roleMenuIDs...)
	}
	menuIDs = UniqueUintSlice(menuIDs)
	menuModel := model.NewMenu()
	menuModel.SetDB(tx)
	enabledMenuIDs, err := menuModel.EnabledIdsByIds(menuIDs)
	if err != nil {
		return nil, err
	}
	enabledMenus := make(map[uint]struct{}, len(enabledMenuIDs))
	for _, menuID := range enabledMenuIDs {
		enabledMenus[menuID] = struct{}{}
	}
	menuApiMapModel := model.NewMenuApiMap()
	menuApiMapModel.SetDB(tx)
	rows, err := menuApiMapModel.MenuApiPermissionsByMenuIds(menuIDs)
	if err != nil {
		return nil, err
	}
	menuPolicies := make(map[uint][][]string, len(menuIDs))
	for _, row := range rows {
		if row.Route != "" && row.Method != "" {
			menuPolicies[row.MenuId] = append(menuPolicies[row.MenuId], []string{row.Route, row.Method})
		}
	}

	for userID, roleIDs := range userExpandedRoles {
		result[userID] = buildUserPolicies(roleIDs, roleMenuMap, enabledMenus, menuPolicies)
	}

	return result, nil
}

func (s *UserPermissionSyncService) userRoleIDs(userID uint, tx ...*gorm.DB) ([]uint, error) {
	roleMapModel := model.NewAdminUserRoleMap()
	deptMapModel := model.NewAdminUserDeptMap()
	deptRoleMapModel := model.NewDeptRoleMap()
	if t := FirstTx(tx); t != nil {
		roleMapModel.SetDB(t)
		deptMapModel.SetDB(t)
		deptRoleMapModel.SetDB(t)
	}

	directRoleIDs, err := roleMapModel.RoleIdsByUid(userID)
	if err != nil {
		return nil, err
	}

	deptIDs, err := deptMapModel.DeptIdsByUid(userID)
	if err != nil {
		return nil, err
	}

	deptRoleIDs, err := deptRoleMapModel.RoleIdsByDeptIds(deptIDs)
	if err != nil {
		return nil, err
	}

	roleModel := model.NewRole()
	if t := FirstTx(tx); t != nil {
		roleModel.SetDB(t)
	}
	return roleModel.EnabledIdsByIds(UniqueUintSlice(append(directRoleIDs, deptRoleIDs...)))
}

// RoleMenuIDs 根据角色列表解析出启用状态的菜单 ID。
func (s *UserPermissionSyncService) RoleMenuIDs(roleIDs []uint, tx ...*gorm.DB) ([]uint, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}

	roleMenuMapModel := model.NewRoleMenuMap()
	menuModel := model.NewMenu()
	if t := FirstTx(tx); t != nil {
		roleMenuMapModel.SetDB(t)
		menuModel.SetDB(t)
	}

	menuIDs, err := roleMenuMapModel.MenuIdsByRoleIds(roleIDs)
	if err != nil || len(menuIDs) == 0 {
		return nil, err
	}

	return menuModel.EnabledIdsByIds(UniqueUintSlice(menuIDs))
}

func (s *UserPermissionSyncService) expandMenuIDsWithParents(menuIDs []uint, tx ...*gorm.DB) ([]uint, error) {
	if len(menuIDs) == 0 {
		return nil, nil
	}

	menuModel := model.NewMenu()
	if t := FirstTx(tx); t != nil {
		menuModel.SetDB(t)
	}

	menus, err := menuModel.FindPidsByIds(menuIDs)
	if err != nil {
		return nil, err
	}

	idSet := make(map[uint]struct{}, len(menuIDs))
	for _, menuID := range menuIDs {
		idSet[menuID] = struct{}{}
	}
	for _, menu := range menus {
		for pid := range strings.SplitSeq(menu.Pids, ",") {
			pid = strings.TrimSpace(pid)
			if pid == "" || pid == "0" {
				continue
			}
			if parsed, err := strconv.ParseUint(pid, 10, 64); err == nil {
				idSet[uint(parsed)] = struct{}{}
			}
		}
	}

	expandedMenuIDs := make([]uint, 0, len(idSet))
	for menuID := range idSet {
		expandedMenuIDs = append(expandedMenuIDs, menuID)
	}
	return menuModel.EnabledIdsByIds(expandedMenuIDs)
}

// UserKey 生成管理员用户对应的 Casbin subject。
func (s *UserPermissionSyncService) UserKey(userID uint) string {
	return fmt.Sprintf("%s%s%d", global.CasbinAdminUserPrefix, global.CasbinSeparator, userID)
}

// -------------------- 角色 / 菜单聚合上下文 --------------------

type roleMenuIDMap map[uint][]uint

func expandUserRoles(userRoleMap map[uint][]uint) (map[uint][]uint, []uint) {
	userExpandedRoles := make(map[uint][]uint, len(userRoleMap))
	allRoleIDs := make([]uint, 0, len(userRoleMap)*2)
	for userID, roleIDs := range userRoleMap {
		expanded := UniqueUintSlice(roleIDs)
		userExpandedRoles[userID] = expanded
		allRoleIDs = append(allRoleIDs, expanded...)
	}
	return userExpandedRoles, UniqueUintSlice(allRoleIDs)
}

func filterUserRolesByEnabledSet(userRoles map[uint][]uint, enabled map[uint]struct{}) (map[uint][]uint, []uint) {
	filtered := make(map[uint][]uint, len(userRoles))
	allRoleIDs := make([]uint, 0, len(userRoles))
	for userID, roleIDs := range userRoles {
		for _, roleID := range roleIDs {
			if _, ok := enabled[roleID]; !ok {
				continue
			}
			filtered[userID] = append(filtered[userID], roleID)
			allRoleIDs = append(allRoleIDs, roleID)
		}
	}
	return filtered, UniqueUintSlice(allRoleIDs)
}

func buildUserPolicies(roleIDs []uint, roleMenuMap roleMenuIDMap, enabledMenus map[uint]struct{}, menuPolicies map[uint][][]string) [][]string {
	menuSet := make(map[uint]struct{})
	for _, roleID := range roleIDs {
		for _, menuID := range roleMenuMap[roleID] {
			if _, ok := enabledMenus[menuID]; ok {
				menuSet[menuID] = struct{}{}
			}
		}
	}
	policies := make([][]string, 0, len(menuSet)*5)
	seenPolicy := make(map[string]struct{})
	for menuID := range menuSet {
		for _, policy := range menuPolicies[menuID] {
			if len(policy) < 2 {
				continue
			}
			key := policy[0] + "::" + policy[1]
			if _, exists := seenPolicy[key]; exists {
				continue
			}
			seenPolicy[key] = struct{}{}
			policies = append(policies, policy)
		}
	}
	return policies
}
