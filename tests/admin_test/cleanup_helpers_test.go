package admin_test

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

// cleanupAdminUsers 清理指定前缀创建的管理员测试数据。
func cleanupAdminUsers(t *testing.T, usernamePrefix string) {
	t.Helper()
	db, err := model.GetDB()
	if err != nil {
		return
	}
	_ = db.Where("username LIKE ?", usernamePrefix+"%").Delete(&model.AdminUser{}).Error
}

// cleanupRoles 清理指定前缀创建的角色测试数据。
func cleanupRoles(t *testing.T, namePrefix string) {
	t.Helper()
	db, err := model.GetDB()
	if err != nil {
		return
	}

	var roles []model.Role
	if err := db.Where("name LIKE ?", namePrefix+"%").Find(&roles).Error; err != nil {
		return
	}
	if len(roles) == 0 {
		return
	}

	ids := make([]uint, 0, len(roles))
	for _, role := range roles {
		ids = append(ids, role.ID)
	}
	_ = db.Where("role_id IN ?", ids).Delete(&model.RoleMenuMap{}).Error
	_ = db.Where("role_id IN ?", ids).Delete(&model.AdminUserRoleMap{}).Error
	_ = db.Where("role_id IN ?", ids).Delete(&model.DeptRoleMap{}).Error
	_ = db.Where("id IN ?", ids).Delete(&model.Role{}).Error
}

// cleanupDepartments 清理指定前缀创建的部门测试数据。
func cleanupDepartments(t *testing.T, namePrefix string) {
	t.Helper()
	db, err := model.GetDB()
	if err != nil {
		return
	}

	var depts []model.Department
	if err := db.Where("name LIKE ?", namePrefix+"%").Find(&depts).Error; err != nil {
		return
	}
	if len(depts) == 0 {
		return
	}

	ids := make([]uint, 0, len(depts))
	for _, dept := range depts {
		ids = append(ids, dept.ID)
	}
	_ = db.Where("dept_id IN ?", ids).Delete(&model.AdminUserDeptMap{}).Error
	_ = db.Where("dept_id IN ?", ids).Delete(&model.DeptRoleMap{}).Error
	_ = db.Where("id IN ?", ids).Delete(&model.Department{}).Error
}

// cleanupMenus 清理指定前缀创建的菜单测试数据。
func cleanupMenus(t *testing.T, titlePrefix string) {
	t.Helper()
	db, err := model.GetDB()
	if err != nil {
		return
	}

	var menus []model.Menu
	if err := db.Where("title LIKE ?", titlePrefix+"%").Find(&menus).Error; err != nil {
		return
	}
	if len(menus) == 0 {
		return
	}

	ids := make([]uint, 0, len(menus))
	for _, menu := range menus {
		ids = append(ids, menu.ID)
	}
	_ = db.Where("menu_id IN ?", ids).Delete(&model.MenuApiMap{}).Error
	_ = db.Where("menu_id IN ?", ids).Delete(&model.RoleMenuMap{}).Error
	_ = db.Where("id IN ?", ids).Delete(&model.Menu{}).Error
}
