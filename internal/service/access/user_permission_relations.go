package access

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"gorm.io/gorm"
)

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

	deptRoleIDs, err := s.deptRoleIDs(db, deptIDs)
	if err != nil {
		return nil, err
	}

	roleIDs := UniqueUintSlice(append(directRoleIDs, deptRoleIDs...))
	return s.expandRoleIDs(roleIDs, tx...)
}

func (s *UserPermissionSyncService) deptRoleIDs(db *gorm.DB, deptIDs []uint) ([]uint, error) {
	if len(deptIDs) == 0 {
		return nil, nil
	}
	return s.QueryUintColumn(db.Table("department_role_map").Where("dept_id IN ?", deptIDs), "role_id")
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

	roleSet := buildAncestorSet(roleIDs, func(add func(uint)) {
		for _, role := range roles {
			addAncestorIDs(role.Pids, add)
		}
	})

	return s.QueryUintColumn(db.Table("role").Where("id IN ? AND status = 1 AND deleted_at = 0", roleSet), "id")
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
	if err != nil || len(menuIDs) == 0 {
		return nil, err
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

	menuSet := buildAncestorSet(menuIDs, func(add func(uint)) {
		for _, menu := range menus {
			addAncestorIDs(menu.Pids, add)
		}
	})

	return s.QueryUintColumn(db.Table("menu").Where("id IN ? AND status = 1 AND deleted_at = 0", menuSet), "id")
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
	type userRoleRow struct {
		UID    uint
		RoleID uint
	}
	var directRows []userRoleRow
	if err := tx.Table("admin_user_role_map").Select("uid,role_id").Where("uid IN ?", userIDs).Scan(&directRows).Error; err != nil {
		return err
	}
	for _, row := range directRows {
		userRoleMap[row.UID] = append(userRoleMap[row.UID], row.RoleID)
	}
	return nil
}

func (s *UserPermissionSyncService) userDepartmentMap(userIDs []uint, tx *gorm.DB) (map[uint][]uint, []uint, error) {
	type userDeptRow struct {
		UID    uint
		DeptID uint
	}
	var deptRows []userDeptRow
	if err := tx.Table("admin_user_department_map").Select("uid,dept_id").Where("uid IN ?", userIDs).Scan(&deptRows).Error; err != nil {
		return nil, nil, err
	}

	deptIDs := make([]uint, 0, len(deptRows))
	userDepts := make(map[uint][]uint, len(deptRows))
	for _, row := range deptRows {
		userDepts[row.UID] = append(userDepts[row.UID], row.DeptID)
		deptIDs = append(deptIDs, row.DeptID)
	}
	return userDepts, UniqueUintSlice(deptIDs), nil
}

func (s *UserPermissionSyncService) departmentRoleMap(deptIDs []uint, tx *gorm.DB) (map[uint][]uint, error) {
	type deptRoleRow struct {
		DeptID uint
		RoleID uint
	}
	var deptRoleRows []deptRoleRow
	if err := tx.Table("department_role_map").Select("dept_id,role_id").Where("dept_id IN ?", deptIDs).Scan(&deptRoleRows).Error; err != nil {
		return nil, err
	}

	deptRoleMap := make(map[uint][]uint, len(deptRoleRows))
	for _, row := range deptRoleRows {
		deptRoleMap[row.DeptID] = append(deptRoleMap[row.DeptID], row.RoleID)
	}
	return deptRoleMap, nil
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
		addAncestorIDs(role.Pids, func(ancestorID uint) {
			if ancestor, ok := roleStatusMap[ancestorID]; ok && ancestor.Status == 1 {
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
