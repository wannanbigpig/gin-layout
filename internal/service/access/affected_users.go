package access

import (
	"strconv"
	"strings"

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
	db, err := resolvePermissionDB(tx...)
	if err != nil {
		return nil, err
	}

	userSet := make([]uint, 0, len(scope.UserIDs))
	userSet = append(userSet, scope.UserIDs...)

	menuIDs := UniqueUintSlice(scope.MenuIDs)
	if len(scope.APIIDs) > 0 {
		apiMenuIDs, err := queryUintColumn(db.Table("menu_api_map").Where("api_id IN ?", scope.APIIDs), "menu_id")
		if err != nil {
			return nil, err
		}
		menuIDs = UniqueUintSlice(append(menuIDs, apiMenuIDs...))
	}

	roleIDs := UniqueUintSlice(scope.RoleIDs)
	if len(menuIDs) > 0 {
		menuRoleIDs, err := queryUintColumn(db.Table("role_menu_map").Where("menu_id IN ?", menuIDs), "role_id")
		if err != nil {
			return nil, err
		}
		roleIDs = UniqueUintSlice(append(roleIDs, menuRoleIDs...))
	}

	if len(roleIDs) > 0 {
		roleUserIDs, err := r.userIDsByRoles(roleIDs, db)
		if err != nil {
			return nil, err
		}
		userSet = append(userSet, roleUserIDs...)
	}

	if len(scope.DepartmentIDs) > 0 {
		deptUserIDs, err := queryUintColumn(db.Table("admin_user_department_map").Where("dept_id IN ?", scope.DepartmentIDs), "uid")
		if err != nil {
			return nil, err
		}
		userSet = append(userSet, deptUserIDs...)
	}

	return UniqueUintSlice(userSet), nil
}

func (r *AffectedUsersResolver) userIDsByRoles(roleIDs []uint, db *gorm.DB) ([]uint, error) {
	roleIDs = UniqueUintSlice(roleIDs)
	if len(roleIDs) == 0 {
		return nil, nil
	}

	expandedRoleIDs, err := r.expandRoleSubtree(roleIDs, db)
	if err != nil {
		return nil, err
	}

	userIDs, err := queryUintColumn(db.Table("admin_user_role_map").Where("role_id IN ?", expandedRoleIDs), "uid")
	if err != nil {
		return nil, err
	}

	deptIDs, err := queryUintColumn(db.Table("department_role_map").Where("role_id IN ?", expandedRoleIDs), "dept_id")
	if err != nil {
		return nil, err
	}
	if len(deptIDs) == 0 {
		return userIDs, nil
	}

	deptUserIDs, err := queryUintColumn(db.Table("admin_user_department_map").Where("dept_id IN ?", deptIDs), "uid")
	if err != nil {
		return nil, err
	}

	return UniqueUintSlice(append(userIDs, deptUserIDs...)), nil
}

func (r *AffectedUsersResolver) expandRoleSubtree(roleIDs []uint, db *gorm.DB) ([]uint, error) {
	roleIDs = UniqueUintSlice(roleIDs)
	if len(roleIDs) == 0 {
		return nil, nil
	}

	type roleTreeNode struct {
		ID   uint
		Pids string
	}

	var roles []roleTreeNode
	if err := db.Table("role").Select("id,pids").Where("deleted_at = 0").Scan(&roles).Error; err != nil {
		return nil, err
	}

	targets := make(map[uint]struct{}, len(roleIDs))
	for _, roleID := range roleIDs {
		targets[roleID] = struct{}{}
	}

	result := make([]uint, 0, len(roleIDs))
	for _, role := range roles {
		if _, ok := targets[role.ID]; ok || containsAnyID(role.Pids, targets) {
			result = append(result, role.ID)
		}
	}

	for _, roleID := range roleIDs {
		if _, ok := targets[roleID]; ok {
			result = append(result, roleID)
		}
	}
	return UniqueUintSlice(result), nil
}

func resolvePermissionDB(tx ...*gorm.DB) (*gorm.DB, error) {
	if db := FirstTx(tx); db != nil {
		return db, nil
	}
	return model.GetDB()
}

func queryUintColumn(db *gorm.DB, column string) ([]uint, error) {
	var values []uint
	if err := db.Pluck(column, &values).Error; err != nil {
		return nil, err
	}
	return UniqueUintSlice(values), nil
}

func containsAnyID(pids string, targets map[uint]struct{}) bool {
	if pids == "" || pids == "0" || len(targets) == 0 {
		return false
	}
	for _, pid := range strings.Split(pids, ",") {
		pid = strings.TrimSpace(pid)
		if pid == "" || pid == "0" {
			continue
		}
		parsed, err := strconv.ParseUint(pid, 10, 64)
		if err != nil {
			continue
		}
		if _, ok := targets[uint(parsed)]; ok {
			return true
		}
	}
	return false
}
