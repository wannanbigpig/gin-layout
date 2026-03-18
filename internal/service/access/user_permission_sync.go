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
		return s.forEachUser(userIDs, func(userID uint) error {
			return s.syncUserWithTx(userID, execTx)
		})
	})
}

// SyncAllUsers 重建全部用户的最终接口权限并同步到 Casbin。
func (s *UserPermissionSyncService) SyncAllUsers(tx ...*gorm.DB) error {
	return s.withSyncTransaction(tx, func(execTx *gorm.DB) error {
		userIDs, err := s.allUserIDs(execTx)
		if err != nil {
			return err
		}
		return s.forEachUser(userIDs, func(userID uint) error {
			return s.syncUserWithTx(userID, execTx)
		})
	})
}

// ClearUser 清理单个用户在 Casbin 中的最终接口权限。
func (s *UserPermissionSyncService) ClearUser(userID uint, tx ...*gorm.DB) error {
	return s.withSyncTransaction(tx, func(execTx *gorm.DB) error {
		enforcer, err := getPolicyEnforcer()
		if err != nil {
			return err
		}
		return enforcer.SetDB(execTx).EditPolicyPermissions(s.userKey(userID), nil)
	})
}

// AccessibleMenuIDs 返回用户可访问的菜单 ID 列表。
// 当 includeParents 为 true 时，会补齐菜单树展示所需的父级目录。
func (s *UserPermissionSyncService) AccessibleMenuIDs(userID uint, includeParents bool, tx ...*gorm.DB) ([]uint, error) {
	roleIDs, err := s.userRoleIDs(userID, tx...)
	if err != nil {
		return nil, err
	}

	menuIDs, err := s.roleMenuIDs(roleIDs, tx...)
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

	menuIDs, err := s.roleMenuIDs(roleIDs, tx...)
	if err != nil {
		return nil, err
	}

	return s.menuAPIPolicies(menuIDs, tx...)
}

// withSyncTransaction 使用现有事务或新事务执行权限同步，确保写入原子性。
func (s *UserPermissionSyncService) withSyncTransaction(tx []*gorm.DB, fn func(execTx *gorm.DB) error) error {
	if existingTx := firstTx(tx); existingTx != nil {
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
	if db := firstTx(tx); db != nil {
		return db, nil
	}
	return model.GetDB()
}

func (s *UserPermissionSyncService) forEachUser(userIDs []uint, fn func(userID uint) error) error {
	for _, userID := range uniqueUintSlice(userIDs) {
		if err := fn(userID); err != nil {
			return err
		}
	}
	return nil
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

	return enforcer.SetDB(tx).EditPolicyPermissions(s.userKey(userID), policies)
}

// userRoleIDs 收集用户直属角色和部门角色，并展开继承角色。
func (s *UserPermissionSyncService) userRoleIDs(userID uint, tx ...*gorm.DB) ([]uint, error) {
	db, err := s.resolveDB(tx)
	if err != nil {
		return nil, err
	}

	directRoleIDs, err := s.queryUintColumn(db.Table("admin_user_role_map").Where("uid = ?", userID), "role_id")
	if err != nil {
		return nil, err
	}

	deptIDs, err := s.queryUintColumn(db.Table("admin_user_department_map").Where("uid = ?", userID), "dept_id")
	if err != nil {
		return nil, err
	}

	deptRoleIDs := make([]uint, 0)
	if len(deptIDs) > 0 {
		deptRoleIDs, err = s.queryUintColumn(db.Table("department_role_map").Where("dept_id IN ?", deptIDs), "role_id")
		if err != nil {
			return nil, err
		}
	}

	roleIDs := uniqueUintSlice(append(directRoleIDs, deptRoleIDs...))
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

	return s.queryUintColumn(db.Table("role").Where("id IN ? AND status = 1 AND deleted_at = 0", allRoleIDs), "id")
}

// roleMenuIDs 根据角色列表解析出启用状态的菜单 ID。
func (s *UserPermissionSyncService) roleMenuIDs(roleIDs []uint, tx ...*gorm.DB) ([]uint, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}

	db, err := s.resolveDB(tx)
	if err != nil {
		return nil, err
	}

	menuIDs, err := s.queryUintColumn(db.Table("role_menu_map").Where("role_id IN ?", roleIDs), "menu_id")
	if err != nil {
		return nil, err
	}
	if len(menuIDs) == 0 {
		return nil, nil
	}

	return s.queryUintColumn(db.Table("menu").Where("id IN ? AND status = 1 AND deleted_at = 0", uniqueUintSlice(menuIDs)), "id")
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

	return s.queryUintColumn(db.Table("menu").Where("id IN ? AND status = 1 AND deleted_at = 0", allMenuIDs), "id")
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
	return s.queryUintColumn(db.Table("admin_user").Where("deleted_at = 0"), "id")
}

// userInfo 加载指定用户 ID 对应的管理员模型。
func (s *UserPermissionSyncService) userInfo(userID uint, tx ...*gorm.DB) (*model.AdminUser, error) {
	user := model.NewAdminUsers()
	if firstTx(tx) != nil {
		user.SetDB(firstTx(tx))
	}
	if err := user.GetById(userID); err != nil {
		return nil, err
	}
	return user, nil
}

// queryUintColumn 提取 uint 列并去重。
func (s *UserPermissionSyncService) queryUintColumn(db *gorm.DB, column string) ([]uint, error) {
	var values []uint
	if err := db.Pluck(column, &values).Error; err != nil {
		return nil, err
	}
	return uniqueUintSlice(values), nil
}

// userKey 生成管理员用户对应的 Casbin subject。
func (s *UserPermissionSyncService) userKey(userID uint) string {
	return fmt.Sprintf("%s%s%d", global.CasbinAdminUserPrefix, global.CasbinSeparator, userID)
}

// uniqueUintSlice 对 uint 切片去重并保留首次出现顺序。
func uniqueUintSlice(values []uint) []uint {
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
