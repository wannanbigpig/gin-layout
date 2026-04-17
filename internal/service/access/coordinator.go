package access

import (
	"gorm.io/gorm"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

// PermissionSyncCoordinator 统一协调权限重建触发逻辑。
type PermissionSyncCoordinator struct {
	syncer   *UserPermissionSyncService
	resolver *AffectedUsersResolver
}

// NewPermissionSyncCoordinator 创建权限同步协调器。
func NewPermissionSyncCoordinator() *PermissionSyncCoordinator {
	return &PermissionSyncCoordinator{
		syncer:   NewUserPermissionSyncService(),
		resolver: NewAffectedUsersResolver(),
	}
}

// SyncAll 重建全部用户最终 API 权限。
func (c *PermissionSyncCoordinator) SyncAll() error {
	if err := NewSystemDefaultsService().Ensure(); err != nil {
		return err
	}
	return c.syncer.SyncAllUsers()
}

// SyncAllInTx 在事务内重建全部用户最终 API 权限。
func (c *PermissionSyncCoordinator) SyncAllInTx(tx *gorm.DB) error {
	if err := NewSystemDefaultsService().Ensure(tx); err != nil {
		return err
	}
	return c.syncer.SyncAllUsers(tx)
}

// SyncUser 重建单个用户最终 API 权限。
func (c *PermissionSyncCoordinator) SyncUser(userID uint, tx ...*gorm.DB) error {
	return c.syncer.SyncUser(userID, tx...)
}

// SyncUsers 重建多个用户最终 API 权限。
func (c *PermissionSyncCoordinator) SyncUsers(userIDs []uint, tx ...*gorm.DB) error {
	return c.syncer.SyncUsers(userIDs, tx...)
}

// SyncUsersAffectedByScope 根据资源变更范围重建受影响用户权限。
func (c *PermissionSyncCoordinator) SyncUsersAffectedByScope(scope PermissionChangeScope, tx ...*gorm.DB) error {
	userIDs, err := c.resolver.Resolve(scope, tx...)
	if err != nil {
		return err
	}
	return c.syncer.SyncUsers(userIDs, tx...)
}

// SyncUsersAffectedByAPIs 重建受指定 API 变更影响的用户权限。
func (c *PermissionSyncCoordinator) SyncUsersAffectedByAPIs(apiIDs []uint, tx ...*gorm.DB) error {
	return c.SyncUsersAffectedByScope(PermissionChangeScope{APIIDs: apiIDs}, tx...)
}

// SyncUsersAffectedByMenus 重建受指定菜单变更影响的用户权限。
func (c *PermissionSyncCoordinator) SyncUsersAffectedByMenus(menuIDs []uint, tx ...*gorm.DB) error {
	return c.SyncUsersAffectedByScope(PermissionChangeScope{MenuIDs: menuIDs}, tx...)
}

// SyncUsersAffectedByRoles 重建受指定角色变更影响的用户权限。
func (c *PermissionSyncCoordinator) SyncUsersAffectedByRoles(roleIDs []uint, tx ...*gorm.DB) error {
	return c.SyncUsersAffectedByScope(PermissionChangeScope{RoleIDs: roleIDs}, tx...)
}

// SyncUsersAffectedByDepartments 重建受指定部门变更影响的用户权限。
func (c *PermissionSyncCoordinator) SyncUsersAffectedByDepartments(deptIDs []uint, tx ...*gorm.DB) error {
	return c.SyncUsersAffectedByScope(PermissionChangeScope{DepartmentIDs: deptIDs}, tx...)
}

// ClearUser 清理单个用户最终 API 权限。
func (c *PermissionSyncCoordinator) ClearUser(userID uint, tx ...*gorm.DB) error {
	return c.syncer.ClearUser(userID, tx...)
}

// AccessibleMenuIDs 返回用户可访问菜单 ID。
func (c *PermissionSyncCoordinator) AccessibleMenuIDs(userID uint, includeParents bool, tx ...*gorm.DB) ([]uint, error) {
	return c.syncer.AccessibleMenuIDs(userID, includeParents, tx...)
}

// ReloadPolicyCache 在事务提交后刷新共享 Casbin Enforcer 的内存策略。
func (c *PermissionSyncCoordinator) ReloadPolicyCache() error {
	return reloadPolicy()
}

// ReloadPolicyCacheWithMessage 在事务提交后刷新共享策略，并统一包装业务错误。
func (c *PermissionSyncCoordinator) ReloadPolicyCacheWithMessage(message string) error {
	if err := c.ReloadPolicyCache(); err != nil {
		return e.NewBusinessError(e.FAILURE, message)
	}
	return nil
}

// RunAfterCommit 执行事务逻辑并在成功提交后刷新共享策略缓存。
func (c *PermissionSyncCoordinator) RunAfterCommit(db *gorm.DB, message string, fn func(tx *gorm.DB) error) error {
	if err := RunInTransaction(db, fn); err != nil {
		return err
	}
	return c.ReloadPolicyCacheWithMessage(message)
}
