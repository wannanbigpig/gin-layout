package admin_test

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

// findAdminUserByUsername 根据用户名查找管理员。
func findAdminUserByUsername(t *testing.T, username string) *model.AdminUser {
	t.Helper()
	user := model.NewAdminUsers()
	db, err := user.GetDB()
	if err != nil {
		t.Fatalf("查询管理员失败: %v", err)
	}
	if err := db.Where("username = ?", username).First(user).Error; err != nil {
		t.Fatalf("查询管理员失败: %v", err)
	}
	return user
}

// findRoleByName 根据角色名称查找角色。
func findRoleByName(t *testing.T, name string) *model.Role {
	t.Helper()
	role := model.NewRole()
	db, err := role.GetDB()
	if err != nil {
		t.Fatalf("查询角色失败: %v", err)
	}
	if err := db.Where("name = ?", name).First(role).Error; err != nil {
		t.Fatalf("查询角色失败: %v", err)
	}
	return role
}

// findDepartmentByName 根据部门名称查找部门。
func findDepartmentByName(t *testing.T, name string) *model.Department {
	t.Helper()
	dept := model.NewDepartment()
	db, err := dept.GetDB()
	if err != nil {
		t.Fatalf("查询部门失败: %v", err)
	}
	if err := db.Where("name = ?", name).First(dept).Error; err != nil {
		t.Fatalf("查询部门失败: %v", err)
	}
	return dept
}

// findMenuByTitle 根据菜单标题查找菜单。
func findMenuByTitle(t *testing.T, title string) *model.Menu {
	t.Helper()
	menu := model.NewMenu()
	db, err := menu.GetDB()
	if err != nil {
		t.Fatalf("查询菜单失败: %v", err)
	}
	if err := db.Where("title = ?", title).First(menu).Error; err != nil {
		t.Fatalf("查询菜单失败: %v", err)
	}
	return menu
}
