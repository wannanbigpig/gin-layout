package access

import (
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"gorm.io/gorm"
)

// collectUserPolicies 根据数据库关系展开用户的最终接口权限。
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

type syncUserRow struct {
	ID           uint
	Status       uint8
	IsSuperAdmin uint8
}

func (s *UserPermissionSyncService) collectActiveUserIDs(userIDs []uint, result map[uint][][]string, tx *gorm.DB) ([]uint, error) {
	var users []syncUserRow
	if err := tx.Table("admin_user").
		Select("id,status,is_super_admin").
		Where("id IN ? AND deleted_at = 0", userIDs).
		Scan(&users).Error; err != nil {
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

func isSyncableUser(userInfo *model.AdminUser) bool {
	return userInfo != nil &&
		userInfo.ID != 0 &&
		userInfo.Status == model.AdminUserStatusEnabled &&
		userInfo.ID != global.SuperAdminId
}

func expandUserRoles(userRoleMap map[uint][]uint, roleStatusMap map[uint]roleStatusInfo) (map[uint][]uint, []uint) {
	userExpandedRoles := make(map[uint][]uint, len(userRoleMap))
	allRoleIDs := make([]uint, 0)
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
	policies := make([][]string, 0)
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
