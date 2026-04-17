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
	userInfo, err := s.userInfo(userID, tx...)
	if err != nil {
		return nil, err
	}
	if !isSyncableUser(userInfo) {
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

	return s.menuAPIPolicies(menuIDs, tx...)
}

// -------------------- 批量用户权限同步链路 --------------------

func (s *UserPermissionSyncService) collectPoliciesForUsers(userIDs []uint, tx *gorm.DB) (map[uint][][]string, error) {
	uniqueIDs := UniqueUintSlice(userIDs)
	result := make(map[uint][][]string, len(uniqueIDs))
	if len(uniqueIDs) == 0 {
		return result, nil
	}

	activeUserIDs, err := s.collectActiveUserIDs(uniqueIDs, result, tx)
	if err != nil || len(activeUserIDs) == 0 {
		return result, err
	}

	userRoleMap, err := s.userBaseRoleMap(activeUserIDs, tx)
	if err != nil || len(userRoleMap) == 0 {
		return result, err
	}

	roleStatusMap, err := s.loadRoleStatusMap(tx)
	if err != nil {
		return nil, err
	}

	userExpandedRoles, allRoleIDs := expandUserRoles(userRoleMap, roleStatusMap)
	if len(allRoleIDs) == 0 {
		return result, nil
	}

	roleMenuMap, err := s.roleMenuMap(allRoleIDs, tx)
	if err != nil || len(roleMenuMap) == 0 {
		return result, err
	}

	enabledMenus, menuPolicies, err := s.collectMenuPermissionData(roleMenuMap.AllMenuIDs(), tx)
	if err != nil {
		return nil, err
	}

	for userID, roleIDs := range userExpandedRoles {
		result[userID] = buildUserPolicies(roleIDs, roleMenuMap, enabledMenus, menuPolicies)
	}

	return result, nil
}

func (s *UserPermissionSyncService) collectActiveUserIDs(userIDs []uint, result map[uint][][]string, tx *gorm.DB) ([]uint, error) {
	userModel := model.NewAdminUsers()
	userModel.SetDB(tx)
	users, err := userModel.SyncUserRows(userIDs)
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
	return activeUserIDs, nil
}

func (s *UserPermissionSyncService) collectMenuPermissionData(menuIDs []uint, tx *gorm.DB) (map[uint]struct{}, map[uint][][]string, error) {
	enabledMenus, err := s.enabledMenuSet(menuIDs, tx)
	if err != nil {
		return nil, nil, err
	}
	menuPolicies, err := s.menuPolicyMap(menuIDs, tx)
	if err != nil {
		return nil, nil, err
	}
	return enabledMenus, menuPolicies, nil
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

	roleIDs := UniqueUintSlice(append(directRoleIDs, deptRoleIDs...))
	return s.expandRoleIDs(roleIDs, tx...)
}

func (s *UserPermissionSyncService) expandRoleIDs(roleIDs []uint, tx ...*gorm.DB) ([]uint, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}

	roleModel := model.NewRole()
	if t := FirstTx(tx); t != nil {
		roleModel.SetDB(t)
	}

	roles, err := roleModel.FindPidsByIds(roleIDs)
	if err != nil {
		return nil, err
	}

	roleSet := buildAncestorSet(roleIDs, func(add func(uint)) {
		for _, role := range roles {
			addAncestorIDs(role.Pids, add)
		}
	})

	return roleModel.EnabledIdsByIds(roleSet)
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

	menuSet := buildAncestorSet(menuIDs, func(add func(uint)) {
		for _, menu := range menus {
			addAncestorIDs(menu.Pids, add)
		}
	})

	return menuModel.EnabledIdsByIds(menuSet)
}

func (s *UserPermissionSyncService) menuAPIPolicies(menuIDs []uint, tx ...*gorm.DB) ([][]string, error) {
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
		if permission.Route == "" || permission.Method == "" {
			continue
		}
		policies = append(policies, []string{permission.Route, permission.Method})
	}
	return policies, nil
}

func (s *UserPermissionSyncService) allUserIDs(tx ...*gorm.DB) ([]uint, error) {
	userModel := model.NewAdminUsers()
	if t := FirstTx(tx); t != nil {
		userModel.SetDB(t)
	}
	return userModel.AllIds()
}

func (s *UserPermissionSyncService) userInfo(userID uint, tx ...*gorm.DB) (*model.AdminUser, error) {
	user := model.NewAdminUsers()
	if FirstTx(tx) != nil {
		user.SetDB(FirstTx(tx))
	}
	if err := user.GetById(userID); err != nil {
		return nil, err
	}
	return user, nil
}

// UserKey 生成管理员用户对应的 Casbin subject。
func (s *UserPermissionSyncService) UserKey(userID uint) string {
	return fmt.Sprintf("%s%s%d", global.CasbinAdminUserPrefix, global.CasbinSeparator, userID)
}

// -------------------- 角色 / 菜单聚合上下文 --------------------

type roleStatusInfo struct {
	ID     uint
	Pids   string
	Status uint8
}

type roleMenuIDMap map[uint][]uint

func (m roleMenuIDMap) AllMenuIDs() []uint {
	menuIDs := make([]uint, 0)
	for _, values := range m {
		menuIDs = append(menuIDs, values...)
	}
	return UniqueUintSlice(menuIDs)
}

func (s *UserPermissionSyncService) userBaseRoleMap(userIDs []uint, tx *gorm.DB) (map[uint][]uint, error) {
	userRoleMap := make(map[uint][]uint, len(userIDs))
	if len(userIDs) == 0 {
		return userRoleMap, nil
	}

	if err := s.appendDirectRoles(userRoleMap, userIDs, tx); err != nil {
		return nil, err
	}

	userDepts, deptIDs, err := s.userDepartmentMap(userIDs, tx)
	if err != nil || len(deptIDs) == 0 {
		return userRoleMap, err
	}

	deptRoleMap, err := s.departmentRoleMap(deptIDs, tx)
	if err != nil {
		return nil, err
	}

	for userID, deptIDs := range userDepts {
		for _, deptID := range deptIDs {
			userRoleMap[userID] = append(userRoleMap[userID], deptRoleMap[deptID]...)
		}
		userRoleMap[userID] = UniqueUintSlice(userRoleMap[userID])
	}

	return userRoleMap, nil
}

func (s *UserPermissionSyncService) appendDirectRoles(userRoleMap map[uint][]uint, userIDs []uint, tx *gorm.DB) error {
	roleMapModel := model.NewAdminUserRoleMap()
	roleMapModel.SetDB(tx)
	directMap, err := roleMapModel.UserRoleMapByUids(userIDs)
	if err != nil {
		return err
	}
	for uid, roleIDs := range directMap {
		userRoleMap[uid] = append(userRoleMap[uid], roleIDs...)
	}
	return nil
}

func (s *UserPermissionSyncService) userDepartmentMap(userIDs []uint, tx *gorm.DB) (map[uint][]uint, []uint, error) {
	deptMapModel := model.NewAdminUserDeptMap()
	deptMapModel.SetDB(tx)
	return deptMapModel.UserDeptMapByUids(userIDs)
}

func (s *UserPermissionSyncService) departmentRoleMap(deptIDs []uint, tx *gorm.DB) (map[uint][]uint, error) {
	deptRoleMapModel := model.NewDeptRoleMap()
	deptRoleMapModel.SetDB(tx)
	return deptRoleMapModel.DeptRoleMapByDeptIds(deptIDs)
}

func (s *UserPermissionSyncService) loadRoleStatusMap(tx *gorm.DB) (map[uint]roleStatusInfo, error) {
	roleModel := model.NewRole()
	roleModel.SetDB(tx)
	rows, err := roleModel.AllRoleStatusInfos()
	if err != nil {
		return nil, err
	}

	roleMap := make(map[uint]roleStatusInfo, len(rows))
	for _, row := range rows {
		roleMap[row.ID] = roleStatusInfo{
			ID:     row.ID,
			Pids:   row.Pids,
			Status: row.Status,
		}
	}
	return roleMap, nil
}

func (s *UserPermissionSyncService) roleMenuMap(roleIDs []uint, tx *gorm.DB) (roleMenuIDMap, error) {
	roleMenuMapModel := model.NewRoleMenuMap()
	roleMenuMapModel.SetDB(tx)
	m, err := roleMenuMapModel.RoleMenuMapByRoleIds(roleIDs)
	if err != nil {
		return nil, err
	}
	return roleMenuIDMap(m), nil
}

func (s *UserPermissionSyncService) enabledMenuSet(menuIDs []uint, tx *gorm.DB) (map[uint]struct{}, error) {
	menuIDs = UniqueUintSlice(menuIDs)
	result := make(map[uint]struct{}, len(menuIDs))
	if len(menuIDs) == 0 {
		return result, nil
	}

	menuModel := model.NewMenu()
	menuModel.SetDB(tx)
	enabledMenuIDs, err := menuModel.EnabledIdsByIds(menuIDs)
	if err != nil {
		return nil, err
	}
	for _, menuID := range enabledMenuIDs {
		result[menuID] = struct{}{}
	}
	return result, nil
}

func (s *UserPermissionSyncService) menuPolicyMap(menuIDs []uint, tx *gorm.DB) (map[uint][][]string, error) {
	menuIDs = UniqueUintSlice(menuIDs)
	result := make(map[uint][][]string, len(menuIDs))
	if len(menuIDs) == 0 {
		return result, nil
	}

	menuApiMapModel := model.NewMenuApiMap()
	menuApiMapModel.SetDB(tx)
	rows, err := menuApiMapModel.MenuApiPermissionsByMenuIds(menuIDs)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		if row.Route == "" || row.Method == "" {
			continue
		}
		result[row.MenuId] = append(result[row.MenuId], []string{row.Route, row.Method})
	}
	return result, nil
}

// -------------------- 树关系与去重工具 --------------------

func buildAncestorSet(baseIDs []uint, collect func(add func(uint))) []uint {
	idSet := make(map[uint]struct{}, len(baseIDs))
	for _, id := range baseIDs {
		idSet[id] = struct{}{}
	}
	collect(func(id uint) {
		idSet[id] = struct{}{}
	})

	result := make([]uint, 0, len(idSet))
	for id := range idSet {
		result = append(result, id)
	}
	return result
}

func addAncestorIDs(pids string, add func(uint)) {
	if pids == "" || pids == "0" {
		return
	}
	for _, pid := range strings.Split(pids, ",") {
		pid = strings.TrimSpace(pid)
		if pid == "" || pid == "0" {
			continue
		}
		if parsed, err := strconv.ParseUint(pid, 10, 64); err == nil {
			add(uint(parsed))
		}
	}
}

func isSyncableUser(userInfo *model.AdminUser) bool {
	return userInfo != nil &&
		userInfo.ID != 0 &&
		userInfo.Status == model.AdminUserStatusEnabled &&
		userInfo.ID != global.SuperAdminId
}

func expandUserRoles(userRoleMap map[uint][]uint, roleStatusMap map[uint]roleStatusInfo) (map[uint][]uint, []uint) {
	userExpandedRoles := make(map[uint][]uint, len(userRoleMap))
	allRoleIDs := make([]uint, 0, len(userRoleMap)*2)
	for userID, roleIDs := range userRoleMap {
		expanded := expandRoleAncestors(roleIDs, roleStatusMap)
		userExpandedRoles[userID] = expanded
		allRoleIDs = append(allRoleIDs, expanded...)
	}
	return userExpandedRoles, UniqueUintSlice(allRoleIDs)
}

func buildUserPolicies(roleIDs []uint, roleMenuMap roleMenuIDMap, enabledMenus map[uint]struct{}, menuPolicies map[uint][][]string) [][]string {
	menuSet := collectEnabledMenuSet(roleIDs, roleMenuMap, enabledMenus)
	return dedupePolicies(menuSet, menuPolicies)
}

func collectEnabledMenuSet(roleIDs []uint, roleMenuMap roleMenuIDMap, enabledMenus map[uint]struct{}) map[uint]struct{} {
	menuSet := make(map[uint]struct{})
	for _, roleID := range roleIDs {
		for _, menuID := range roleMenuMap[roleID] {
			if _, ok := enabledMenus[menuID]; ok {
				menuSet[menuID] = struct{}{}
			}
		}
	}
	return menuSet
}

func dedupePolicies(menuSet map[uint]struct{}, menuPolicies map[uint][][]string) [][]string {
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

func expandRoleAncestors(roleIDs []uint, roleStatusMap map[uint]roleStatusInfo) []uint {
	roleSet := make(map[uint]struct{})
	for _, roleID := range UniqueUintSlice(roleIDs) {
		role, ok := roleStatusMap[roleID]
		if !ok || role.Status != global.Yes {
			continue
		}
		roleSet[roleID] = struct{}{}
		addAncestorIDs(role.Pids, func(ancestorID uint) {
			if ancestor, ok := roleStatusMap[ancestorID]; ok && ancestor.Status == global.Yes {
				roleSet[ancestorID] = struct{}{}
			}
		})
	}

	result := make([]uint, 0, len(roleSet))
	for roleID := range roleSet {
		result = append(result, roleID)
	}
	return UniqueUintSlice(result)
}
