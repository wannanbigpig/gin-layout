package access

import (
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

// PermissionChangeScope 描述一次业务变更影响的权限对象范围。
type PermissionChangeScope struct {
	APIIDs        []uint
	MenuIDs       []uint
	RoleIDs       []uint
	DepartmentIDs []uint
	UserIDs       []uint
}

// AffectedUsersResolver 负责将资源变更转换成受影响用户集合。
type AffectedUsersResolver struct{}

// NewAffectedUsersResolver 创建受影响用户解析器。
func NewAffectedUsersResolver() *AffectedUsersResolver {
	return &AffectedUsersResolver{}
}

// Resolve 返回指定作用域下的受影响用户集合。
func (r *AffectedUsersResolver) Resolve(scope PermissionChangeScope, tx ...*gorm.DB) ([]uint, error) {
	userSet := make([]uint, 0, len(scope.UserIDs))
	userSet = append(userSet, scope.UserIDs...)

	menuIDs := UniqueUintSlice(scope.MenuIDs)
	if len(scope.APIIDs) > 0 {
		menuApiMapModel := model.NewMenuApiMap()
		if t := FirstTx(tx); t != nil {
			menuApiMapModel.SetDB(t)
		}
		apiMenuIDs, err := menuApiMapModel.MenuIdsByApiIds(scope.APIIDs)
		if err != nil {
			return nil, err
		}
		menuIDs = UniqueUintSlice(append(menuIDs, apiMenuIDs...))
	}

	roleIDs := UniqueUintSlice(scope.RoleIDs)
	if len(menuIDs) > 0 {
		roleMenuMapModel := model.NewRoleMenuMap()
		if t := FirstTx(tx); t != nil {
			roleMenuMapModel.SetDB(t)
		}
		menuRoleIDs, err := roleMenuMapModel.RoleIdsByMenuIds(menuIDs)
		if err != nil {
			return nil, err
		}
		roleIDs = UniqueUintSlice(append(roleIDs, menuRoleIDs...))
	}

	if len(roleIDs) > 0 {
		roleUserIDs, err := r.userIDsByRoles(roleIDs, tx...)
		if err != nil {
			return nil, err
		}
		userSet = append(userSet, roleUserIDs...)
	}

	if len(scope.DepartmentIDs) > 0 {
		deptMapModel := model.NewAdminUserDeptMap()
		if t := FirstTx(tx); t != nil {
			deptMapModel.SetDB(t)
		}
		deptUserIDs, err := deptMapModel.UidsByDeptIds(scope.DepartmentIDs)
		if err != nil {
			return nil, err
		}
		userSet = append(userSet, deptUserIDs...)
	}

	return UniqueUintSlice(userSet), nil
}

func (r *AffectedUsersResolver) userIDsByRoles(roleIDs []uint, tx ...*gorm.DB) ([]uint, error) {
	roleIDs = UniqueUintSlice(roleIDs)
	if len(roleIDs) == 0 {
		return nil, nil
	}

	expandedRoleIDs, err := r.expandRoleSubtree(roleIDs, tx...)
	if err != nil {
		return nil, err
	}

	roleMapModel := model.NewAdminUserRoleMap()
	deptRoleMapModel := model.NewDeptRoleMap()
	deptMapModel := model.NewAdminUserDeptMap()
	if t := FirstTx(tx); t != nil {
		roleMapModel.SetDB(t)
		deptRoleMapModel.SetDB(t)
		deptMapModel.SetDB(t)
	}

	userIDs, err := roleMapModel.UidsByRoleIds(expandedRoleIDs)
	if err != nil {
		return nil, err
	}

	deptIDs, err := deptRoleMapModel.DeptIdsByRoleIds(expandedRoleIDs)
	if err != nil {
		return nil, err
	}
	if len(deptIDs) == 0 {
		return userIDs, nil
	}

	deptUserIDs, err := deptMapModel.UidsByDeptIds(deptIDs)
	if err != nil {
		return nil, err
	}

	return UniqueUintSlice(append(userIDs, deptUserIDs...)), nil
}

func (r *AffectedUsersResolver) expandRoleSubtree(roleIDs []uint, tx ...*gorm.DB) ([]uint, error) {
	roleIDs = UniqueUintSlice(roleIDs)
	if len(roleIDs) == 0 {
		return nil, nil
	}

	roleModel := model.NewRole()
	if t := FirstTx(tx); t != nil {
		roleModel.SetDB(t)
	}
	subtreeIDs, err := roleModel.SubtreeIdsByRootIds(roleIDs)
	if err != nil {
		return nil, err
	}
	return UniqueUintSlice(append(subtreeIDs, roleIDs...)), nil
}
