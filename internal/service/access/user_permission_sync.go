package access

import (
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	casbinx "github.com/wannanbigpig/gin-layout/internal/access/casbin"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

// UserPermissionSyncService 负责把数据库关系展开为用户最终接口权限。
type UserPermissionSyncService struct{}

// NewUserPermissionSyncService 创建用户权限同步服务实例。
func NewUserPermissionSyncService() *UserPermissionSyncService {
	return &UserPermissionSyncService{}
}

// SyncUser 重建单个用户的最终接口权限并同步到 Casbin。
func (s *UserPermissionSyncService) SyncUser(userID uint, tx ...*gorm.DB) error {
	return s.withSyncTransaction(tx, func(execTx *gorm.DB) error { return s.syncUserWithTx(userID, execTx) })
}

// SyncUsers 重建多个用户的最终接口权限并同步到 Casbin。
func (s *UserPermissionSyncService) SyncUsers(userIDs []uint, tx ...*gorm.DB) error {
	return s.withSyncTransaction(tx, func(execTx *gorm.DB) error {
		enforcer, err := getPolicyEnforcer()
		if err != nil {
			return err
		}
		return s.batchSyncUsersWithEnforcer(userIDs, enforcer, execTx)
	})
}

// SyncAllUsers 重建全部用户的最终接口权限并同步到 Casbin。
func (s *UserPermissionSyncService) SyncAllUsers(tx ...*gorm.DB) error {
	return s.withSyncTransaction(tx, func(execTx *gorm.DB) error {
		userIDs, err := s.allUserIDs(execTx)
		if err != nil {
			return err
		}
		enforcer, err := getPolicyEnforcer()
		if err != nil {
			return err
		}
		return s.batchSyncUsersWithEnforcer(userIDs, enforcer, execTx)
	})
}

// ClearUser 清理单个用户在 Casbin 中的最终接口权限。
func (s *UserPermissionSyncService) ClearUser(userID uint, tx ...*gorm.DB) error {
	return s.withSyncTransaction(tx, func(execTx *gorm.DB) error {
		enforcer, err := getPolicyEnforcer()
		if err != nil {
			return err
		}
		return enforcer.SetDB(execTx).EditPolicyPermissions(s.UserKey(userID), nil)
	})
}

// AccessibleMenuIDs 返回用户可访问的菜单 ID 列表。
// 当 includeParents 为 true 时，会补齐菜单树展示所需的父级目录。
func (s *UserPermissionSyncService) AccessibleMenuIDs(userID uint, includeParents bool, tx ...*gorm.DB) ([]uint, error) {
	roleIDs, err := s.userRoleIDs(userID, tx...)
	if err != nil {
		return nil, err
	}

	menuIDs, err := s.RoleMenuIDs(roleIDs, tx...)
	if err != nil {
		return nil, err
	}

	if includeParents {
		return s.expandMenuIDsWithParents(menuIDs, tx...)
	}
	return menuIDs, nil
}

// collectUserPolicies 根据数据库关系展开用户的最终接口权限。
func (s *UserPermissionSyncService) collectUserPolicies(userID uint, tx ...*gorm.DB) ([][]string, error) {
	userInfo, err := s.userInfo(userID, tx...)
	if err != nil {
		return nil, err
	}

	if userInfo == nil || userInfo.ID == 0 || userInfo.Status != model.AdminUserStatusEnabled || userInfo.ID == global.SuperAdminId {
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

	type userRow struct {
		ID           uint
		Status       uint8
		IsSuperAdmin uint8
	}
	var users []userRow
	if err := tx.Table("admin_user").
		Select("id,status,is_super_admin").
		Where("id IN ? AND deleted_at = 0", uniqueIDs).
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
	if len(activeUserIDs) == 0 {
		return result, nil
	}

	userRoleMap, err := s.userBaseRoleMap(activeUserIDs, tx)
	if err != nil {
		return nil, err
	}
	if len(userRoleMap) == 0 {
		return result, nil
	}

	roleStatusMap, err := s.loadRoleStatusMap(tx)
	if err != nil {
		return nil, err
	}

	userExpandedRoles := make(map[uint][]uint, len(userRoleMap))
	allRoleIDs := make([]uint, 0)
	for userID, roleIDs := range userRoleMap {
		expanded := expandRoleAncestors(roleIDs, roleStatusMap)
		userExpandedRoles[userID] = expanded
		allRoleIDs = append(allRoleIDs, expanded...)
	}
	allRoleIDs = UniqueUintSlice(allRoleIDs)
	if len(allRoleIDs) == 0 {
		return result, nil
	}

	roleMenuMap, err := s.roleMenuMap(allRoleIDs, tx)
	if err != nil {
		return nil, err
	}
	if len(roleMenuMap) == 0 {
		return result, nil
	}

	enabledMenus, err := s.enabledMenuSet(roleMenuMap.AllMenuIDs(), tx)
	if err != nil {
		return nil, err
	}
	menuPolicies, err := s.menuPolicyMap(roleMenuMap.AllMenuIDs(), tx)
	if err != nil {
		return nil, err
	}

	for userID, roleIDs := range userExpandedRoles {
		menuSet := make(map[uint]struct{})
		for _, roleID := range roleIDs {
			for _, menuID := range roleMenuMap[roleID] {
				if _, ok := enabledMenus[menuID]; ok {
					menuSet[menuID] = struct{}{}
				}
			}
		}

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
		result[userID] = policies
	}

	return result, nil
}

// withSyncTransaction 使用现有事务或新事务执行权限同步，确保写入原子性。
func (s *UserPermissionSyncService) withSyncTransaction(tx []*gorm.DB, fn func(execTx *gorm.DB) error) error {
	if existingTx := FirstTx(tx); existingTx != nil {
		return fn(existingTx)
	}
	db, err := s.resolveDB(nil)
	if err != nil {
		return err
	}
	if err := db.Transaction(fn); err != nil {
		return err
	}
	return casbinx.ReloadPolicy()
}

func (s *UserPermissionSyncService) resolveDB(tx []*gorm.DB) (*gorm.DB, error) {
	if db := FirstTx(tx); db != nil {
		return db, nil
	}
	return model.GetDB()
}

func (s *UserPermissionSyncService) forEachUser(userIDs []uint, fn func(userID uint) error) error {
	uniqueIDs := UniqueUintSlice(userIDs)
	if len(uniqueIDs) == 0 {
		return nil
	}
	for _, userID := range uniqueIDs {
		if err := fn(userID); err != nil {
			return err
		}
	}
	return nil
}

// batchSyncUsersWithEnforcer 批量同步多个用户的权限，使用同一个enforcer减少重复获取
func (s *UserPermissionSyncService) batchSyncUsersWithEnforcer(userIDs []uint, enforcer *casbinx.CasbinEnforcer, tx *gorm.DB) error {
	uniqueIDs := UniqueUintSlice(userIDs)
	if len(uniqueIDs) == 0 {
		return nil
	}

	policiesByUser, err := s.collectPoliciesForUsers(uniqueIDs, tx)
	if err != nil {
		return err
	}

	subjectPolicies := make(map[string][][]string, len(uniqueIDs))
	for _, userID := range uniqueIDs {
		subjectPolicies[s.UserKey(userID)] = policiesByUser[userID]
	}

	return enforcer.EditPolicyPermissionsBatch(subjectPolicies, tx)
}

// syncUserWithTx 在指定事务内同步单个用户的最终接口权限。
func (s *UserPermissionSyncService) syncUserWithTx(userID uint, tx *gorm.DB) error {
	enforcer, err := getPolicyEnforcer()
	if err != nil {
		return err
	}

	policies, err := s.collectUserPolicies(userID, tx)
	if err != nil {
		return err
	}

	return enforcer.SetDB(tx).EditPolicyPermissions(s.UserKey(userID), policies)
}

// userRoleIDs 收集用户直属角色和部门角色，并展开继承角色。
func (s *UserPermissionSyncService) userRoleIDs(userID uint, tx ...*gorm.DB) ([]uint, error) {
	db, err := s.resolveDB(tx)
	if err != nil {
		return nil, err
	}

	directRoleIDs, err := s.QueryUintColumn(db.Table("admin_user_role_map").Where("uid = ?", userID), "role_id")
	if err != nil {
		return nil, err
	}

	deptIDs, err := s.QueryUintColumn(db.Table("admin_user_department_map").Where("uid = ?", userID), "dept_id")
	if err != nil {
		return nil, err
	}

	deptRoleIDs := make([]uint, 0)
	if len(deptIDs) > 0 {
		deptRoleIDs, err = s.QueryUintColumn(db.Table("department_role_map").Where("dept_id IN ?", deptIDs), "role_id")
		if err != nil {
			return nil, err
		}
	}

	roleIDs := UniqueUintSlice(append(directRoleIDs, deptRoleIDs...))
	return s.expandRoleIDs(roleIDs, tx...)
}

// expandRoleIDs 根据角色的 pids 链补齐启用状态的祖先角色。
func (s *UserPermissionSyncService) expandRoleIDs(roleIDs []uint, tx ...*gorm.DB) ([]uint, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}

	db, err := s.resolveDB(tx)
	if err != nil {
		return nil, err
	}

	var roles []*model.Role
	if err := db.Where("id IN ? AND deleted_at = 0", roleIDs).Find(&roles).Error; err != nil {
		return nil, err
	}

	roleSet := make(map[uint]struct{}, len(roleIDs))
	for _, roleID := range roleIDs {
		roleSet[roleID] = struct{}{}
	}

	for _, role := range roles {
		if role.Pids == "" || role.Pids == "0" {
			continue
		}
		for _, pid := range strings.Split(role.Pids, ",") {
			pid = strings.TrimSpace(pid)
			if pid == "" || pid == "0" {
				continue
			}
			if parsed, err := strconv.ParseUint(pid, 10, 64); err == nil {
				roleSet[uint(parsed)] = struct{}{}
			}
		}
	}

	allRoleIDs := make([]uint, 0, len(roleSet))
	for roleID := range roleSet {
		allRoleIDs = append(allRoleIDs, roleID)
	}

	return s.QueryUintColumn(db.Table("role").Where("id IN ? AND status = 1 AND deleted_at = 0", allRoleIDs), "id")
}

// RoleMenuIDs 根据角色列表解析出启用状态的菜单 ID。
func (s *UserPermissionSyncService) RoleMenuIDs(roleIDs []uint, tx ...*gorm.DB) ([]uint, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}

	db, err := s.resolveDB(tx)
	if err != nil {
		return nil, err
	}

	menuIDs, err := s.QueryUintColumn(db.Table("role_menu_map").Where("role_id IN ?", roleIDs), "menu_id")
	if err != nil {
		return nil, err
	}
	if len(menuIDs) == 0 {
		return nil, nil
	}

	return s.QueryUintColumn(db.Table("menu").Where("id IN ? AND status = 1 AND deleted_at = 0", UniqueUintSlice(menuIDs)), "id")
}

// expandMenuIDsWithParents 为菜单集合补齐祖先菜单 ID。
func (s *UserPermissionSyncService) expandMenuIDsWithParents(menuIDs []uint, tx ...*gorm.DB) ([]uint, error) {
	if len(menuIDs) == 0 {
		return nil, nil
	}

	db, err := s.resolveDB(tx)
	if err != nil {
		return nil, err
	}

	var menus []*model.Menu
	if err := db.Select("id,pids").Where("id IN ? AND deleted_at = 0", menuIDs).Find(&menus).Error; err != nil {
		return nil, err
	}

	menuSet := make(map[uint]struct{}, len(menuIDs))
	for _, menuID := range menuIDs {
		menuSet[menuID] = struct{}{}
	}

	for _, menu := range menus {
		if menu.Pids == "" || menu.Pids == "0" {
			continue
		}
		for _, pid := range strings.Split(menu.Pids, ",") {
			pid = strings.TrimSpace(pid)
			if pid == "" || pid == "0" {
				continue
			}
			if parsed, err := strconv.ParseUint(pid, 10, 64); err == nil {
				menuSet[uint(parsed)] = struct{}{}
			}
		}
	}

	allMenuIDs := make([]uint, 0, len(menuSet))
	for menuID := range menuSet {
		allMenuIDs = append(allMenuIDs, menuID)
	}

	return s.QueryUintColumn(db.Table("menu").Where("id IN ? AND status = 1 AND deleted_at = 0", allMenuIDs), "id")
}

// menuAPIPolicies 将菜单与接口关系转换为 Casbin 权限元组。
func (s *UserPermissionSyncService) menuAPIPolicies(menuIDs []uint, tx ...*gorm.DB) ([][]string, error) {
	if len(menuIDs) == 0 {
		return nil, nil
	}

	db, err := s.resolveDB(tx)
	if err != nil {
		return nil, err
	}

	type apiPermission struct {
		Route  string
		Method string
	}

	var permissions []apiPermission
	err = db.Table("menu_api_map m").
		Select("DISTINCT a.route, a.method").
		Joins("JOIN api a ON a.id = m.api_id").
		Where("m.menu_id IN ? AND a.deleted_at = 0 AND a.is_auth = 1 AND a.is_effective = 1", menuIDs).
		Find(&permissions).Error
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

// allUserIDs 返回全部未删除的管理员用户 ID。
func (s *UserPermissionSyncService) allUserIDs(tx ...*gorm.DB) ([]uint, error) {
	db, err := s.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	return s.QueryUintColumn(db.Table("admin_user").Where("deleted_at = 0"), "id")
}

// userInfo 加载指定用户 ID 对应的管理员模型。
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

// QueryUintColumn 提取 uint 列并去重。
func (s *UserPermissionSyncService) QueryUintColumn(db *gorm.DB, column string) ([]uint, error) {
	return queryUintColumn(db, column)
}

// UserKey 生成管理员用户对应的 Casbin subject。
func (s *UserPermissionSyncService) UserKey(userID uint) string {
	return fmt.Sprintf("%s%s%d", global.CasbinAdminUserPrefix, global.CasbinSeparator, userID)
}

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

	type userRoleRow struct {
		UID    uint
		RoleID uint
	}
	var directRows []userRoleRow
	if err := tx.Table("admin_user_role_map").Select("uid,role_id").Where("uid IN ?", userIDs).Scan(&directRows).Error; err != nil {
		return nil, err
	}
	for _, row := range directRows {
		userRoleMap[row.UID] = append(userRoleMap[row.UID], row.RoleID)
	}

	type userDeptRow struct {
		UID    uint
		DeptID uint
	}
	var deptRows []userDeptRow
	if err := tx.Table("admin_user_department_map").Select("uid,dept_id").Where("uid IN ?", userIDs).Scan(&deptRows).Error; err != nil {
		return nil, err
	}
	if len(deptRows) == 0 {
		return userRoleMap, nil
	}

	deptIDs := make([]uint, 0, len(deptRows))
	userDepts := make(map[uint][]uint, len(deptRows))
	for _, row := range deptRows {
		userDepts[row.UID] = append(userDepts[row.UID], row.DeptID)
		deptIDs = append(deptIDs, row.DeptID)
	}

	type deptRoleRow struct {
		DeptID uint
		RoleID uint
	}
	var deptRoleRows []deptRoleRow
	if err := tx.Table("department_role_map").Select("dept_id,role_id").Where("dept_id IN ?", UniqueUintSlice(deptIDs)).Scan(&deptRoleRows).Error; err != nil {
		return nil, err
	}

	deptRoleMap := make(map[uint][]uint, len(deptRoleRows))
	for _, row := range deptRoleRows {
		deptRoleMap[row.DeptID] = append(deptRoleMap[row.DeptID], row.RoleID)
	}

	for userID, deptIDs := range userDepts {
		for _, deptID := range deptIDs {
			userRoleMap[userID] = append(userRoleMap[userID], deptRoleMap[deptID]...)
		}
		userRoleMap[userID] = UniqueUintSlice(userRoleMap[userID])
	}

	return userRoleMap, nil
}

func (s *UserPermissionSyncService) loadRoleStatusMap(tx *gorm.DB) (map[uint]roleStatusInfo, error) {
	var rows []roleStatusInfo
	if err := tx.Table("role").Select("id,pids,status").Where("deleted_at = 0").Scan(&rows).Error; err != nil {
		return nil, err
	}

	roleMap := make(map[uint]roleStatusInfo, len(rows))
	for _, row := range rows {
		roleMap[row.ID] = row
	}
	return roleMap, nil
}

func (s *UserPermissionSyncService) roleMenuMap(roleIDs []uint, tx *gorm.DB) (roleMenuIDMap, error) {
	result := make(roleMenuIDMap)
	if len(roleIDs) == 0 {
		return result, nil
	}

	type roleMenuRow struct {
		RoleID uint
		MenuID uint
	}
	var rows []roleMenuRow
	if err := tx.Table("role_menu_map").Select("role_id,menu_id").Where("role_id IN ?", roleIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.RoleID] = append(result[row.RoleID], row.MenuID)
	}
	return result, nil
}

func (s *UserPermissionSyncService) enabledMenuSet(menuIDs []uint, tx *gorm.DB) (map[uint]struct{}, error) {
	menuIDs = UniqueUintSlice(menuIDs)
	result := make(map[uint]struct{}, len(menuIDs))
	if len(menuIDs) == 0 {
		return result, nil
	}

	enabledMenuIDs, err := queryUintColumn(tx.Table("menu").Where("id IN ? AND status = 1 AND deleted_at = 0", menuIDs), "id")
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

	type menuPermissionRow struct {
		MenuID uint
		Route  string
		Method string
	}
	var rows []menuPermissionRow
	err := tx.Table("menu_api_map m").
		Select("m.menu_id, a.route, a.method").
		Joins("JOIN api a ON a.id = m.api_id").
		Where("m.menu_id IN ? AND a.deleted_at = 0 AND a.is_auth = 1 AND a.is_effective = 1", menuIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		if row.Route == "" || row.Method == "" {
			continue
		}
		result[row.MenuID] = append(result[row.MenuID], []string{row.Route, row.Method})
	}
	return result, nil
}

func expandRoleAncestors(roleIDs []uint, roleStatusMap map[uint]roleStatusInfo) []uint {
	roleSet := make(map[uint]struct{})
	for _, roleID := range UniqueUintSlice(roleIDs) {
		role, ok := roleStatusMap[roleID]
		if !ok || role.Status != 1 {
			continue
		}
		roleSet[roleID] = struct{}{}
		for _, pid := range strings.Split(role.Pids, ",") {
			pid = strings.TrimSpace(pid)
			if pid == "" || pid == "0" {
				continue
			}
			parsed, err := strconv.ParseUint(pid, 10, 64)
			if err != nil {
				continue
			}
			ancestorID := uint(parsed)
			if ancestor, ok := roleStatusMap[ancestorID]; ok && ancestor.Status == 1 {
				roleSet[ancestorID] = struct{}{}
			}
		}
	}

	result := make([]uint, 0, len(roleSet))
	for roleID := range roleSet {
		result = append(result, roleID)
	}
	return UniqueUintSlice(result)
}

// UniqueUintSlice 对 uint 切片去重并保留首次出现顺序。
func UniqueUintSlice(values []uint) []uint {
	if len(values) == 0 {
		return nil
	}
	set := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if _, ok := set[value]; ok {
			continue
		}
		set[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
