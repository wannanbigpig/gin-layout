package admin_test

import (
	"testing"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

// firstActiveRoleID 返回一个可用于绑定的启用角色 ID。
func firstActiveRoleID(t *testing.T) uint {
	t.Helper()
	role := model.NewRole()
	db, err := role.GetDB()
	if err != nil {
		t.Fatalf("查询启用角色失败: %v", err)
	}
	if err := db.Where("status = 1").First(role).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return createFallbackRole(t)
		}
		t.Fatalf("查询启用角色失败: %v", err)
	}
	return role.ID
}

// firstActiveMenuID 返回一个可用于角色绑定的启用菜单 ID。
func firstActiveMenuID(t *testing.T) uint {
	t.Helper()
	menu := model.NewMenu()
	db, err := menu.GetDB()
	if err != nil {
		t.Fatalf("查询启用菜单失败: %v", err)
	}
	if err := db.Where("status = 1").First(menu).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return createFallbackMenu(t)
		}
		t.Fatalf("查询启用菜单失败: %v", err)
	}
	return menu.ID
}

// createFallbackRole 创建测试兜底角色。
func createFallbackRole(t *testing.T) uint {
	t.Helper()
	name := uniqueCompactTestName("role-seed")
	role := &model.Role{
		Name:   name,
		Status: 1,
		Level:  1,
		Pids:   "0",
		Sort:   1,
	}
	db, err := role.GetDB()
	if err != nil {
		t.Fatalf("创建兜底角色失败: %v", err)
	}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("创建兜底角色失败: %v", err)
	}
	return role.ID
}

// createFallbackMenu 创建测试兜底菜单。
func createFallbackMenu(t *testing.T) uint {
	t.Helper()
	name := uniqueCompactTestName("menu")
	menu := &model.Menu{
		Title:     name,
		Name:      name,
		Path:      "/" + name,
		FullPath:  "/" + name,
		Component: "test/component",
		IsShow:    1,
		Sort:      1,
		Type:      model.MENU,
		Level:     1,
		Pids:      "0",
		IsAuth:    1,
		Status:    1,
	}
	db, err := menu.GetDB()
	if err != nil {
		t.Fatalf("创建兜底菜单失败: %v", err)
	}
	if err := db.Create(menu).Error; err != nil {
		t.Fatalf("创建兜底菜单失败: %v", err)
	}
	return menu.ID
}
