package access

import (
	"gorm.io/gorm"

	casbinx "github.com/wannanbigpig/gin-layout/internal/access/casbin"
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

// withSyncTransaction 使用现有事务或新事务执行权限同步，确保写入原子性。
func (s *UserPermissionSyncService) withSyncTransaction(tx []*gorm.DB, fn func(execTx *gorm.DB) error) error {
	if existingTx := FirstTx(tx); existingTx != nil {
		return fn(existingTx)
	}
	db, err := model.NewAdminUsers().GetDB()
	if err != nil {
		return err
	}
	if err := db.Transaction(fn); err != nil {
		return err
	}
	return reloadPolicy()
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
