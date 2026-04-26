package admin_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
)

const testResourcePrefix = "test-auto-"

// requireWritableDB 在需要真实数据库写入时跳过测试。
func requireWritableDB(t *testing.T) {
	t.Helper()
	requireMySQL(t)
	if _, err := model.GetDB(); err != nil {
		t.Skip("数据库连接不可用，跳过真实写入测试")
	}
}

// uniqueTestName 生成用于测试资源的唯一名称。
func uniqueTestName(kind string) string {
	return fmt.Sprintf("%s%s-%d", testResourcePrefix, kind, time.Now().UnixNano())
}

// containsPrefix 判断字符串是否包含测试前缀。
func containsPrefix(s string) bool {
	return strings.HasPrefix(s, testResourcePrefix)
}

// uniqueCompactTestName 生成适合表单校验长度限制的测试名称。
func uniqueCompactTestName(kind string) string {
	return fmt.Sprintf("ta%s%d", strings.ReplaceAll(kind, "-", ""), time.Now().UnixNano()%1e8)
}

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
	db, err := model.GetDB()
	if err != nil {
		t.Fatalf("查询菜单失败: %v", err)
	}
	translation := model.NewMenuI18n()
	if err := db.Where("locale = ? AND title = ?", i18n.LocaleZhCN, title).First(translation).Error; err != nil {
		t.Fatalf("查询菜单翻译失败: %v", err)
	}
	menu := model.NewMenu()
	if err := db.Where("id = ?", translation.MenuID).First(menu).Error; err != nil {
		t.Fatalf("查询菜单失败: %v", err)
	}
	return menu
}

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

	var ids []uint
	if err := db.Model(&model.MenuI18n{}).
		Where("locale = ? AND title LIKE ?", i18n.LocaleZhCN, titlePrefix+"%").
		Distinct().
		Pluck("menu_id", &ids).Error; err != nil {
		return
	}
	if len(ids) == 0 {
		return
	}
	_ = db.Where("menu_id IN ?", ids).Delete(&model.MenuApiMap{}).Error
	_ = db.Where("menu_id IN ?", ids).Delete(&model.RoleMenuMap{}).Error
	_ = db.Where("menu_id IN ?", ids).Delete(&model.MenuI18n{}).Error
	_ = db.Where("id IN ?", ids).Delete(&model.Menu{}).Error
}

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
	if err := model.NewMenuI18n().UpsertMenuTitles(menu.ID, map[string]string{
		i18n.LocaleZhCN: name,
		i18n.LocaleEnUS: name,
	}, db); err != nil {
		t.Fatalf("创建兜底菜单翻译失败: %v", err)
	}
	return menu.ID
}

// findSysConfigByKey 根据参数键名查找系统参数。
func findSysConfigByKey(t *testing.T, key string) *model.SysConfig {
	t.Helper()
	config := model.NewSysConfig()
	db, err := config.GetDB()
	if err != nil {
		t.Fatalf("查询系统参数失败: %v", err)
	}
	if err := db.Where("config_key = ? AND deleted_at = 0", key).First(config).Error; err != nil {
		t.Fatalf("查询系统参数失败: %v", err)
	}
	return config
}

// findSysDictTypeByCode 根据字典类型编码查找字典类型。
func findSysDictTypeByCode(t *testing.T, typeCode string) *model.SysDictType {
	t.Helper()
	dictType := model.NewSysDictType()
	db, err := dictType.GetDB()
	if err != nil {
		t.Fatalf("查询字典类型失败: %v", err)
	}
	if err := db.Where("type_code = ? AND deleted_at = 0", typeCode).First(dictType).Error; err != nil {
		t.Fatalf("查询字典类型失败: %v", err)
	}
	return dictType
}

// findSysDictItemByTypeAndValue 根据类型编码与字典值查找字典项。
func findSysDictItemByTypeAndValue(t *testing.T, typeCode, value string) *model.SysDictItem {
	t.Helper()
	item := model.NewSysDictItem()
	db, err := item.GetDB()
	if err != nil {
		t.Fatalf("查询字典项失败: %v", err)
	}
	if err := db.Where("type_code = ? AND value = ? AND deleted_at = 0", typeCode, value).First(item).Error; err != nil {
		t.Fatalf("查询字典项失败: %v", err)
	}
	return item
}

// cleanupSysConfigsByKeyPrefix 清理指定参数键前缀的系统参数测试数据。
func cleanupSysConfigsByKeyPrefix(t *testing.T, keyPrefix string) {
	t.Helper()
	db, err := model.GetDB()
	if err != nil {
		return
	}
	var configs []model.SysConfig
	if err := db.Unscoped().Where("config_key LIKE ?", keyPrefix+"%").Find(&configs).Error; err != nil {
		return
	}
	if len(configs) == 0 {
		return
	}
	ids := make([]uint, 0, len(configs))
	for _, config := range configs {
		ids = append(ids, config.ID)
	}
	_ = db.Unscoped().Where("config_id IN ?", ids).Delete(&model.SysConfigI18n{}).Error
	_ = db.Unscoped().Where("id IN ?", ids).Delete(&model.SysConfig{}).Error
}

// cleanupSysDictByTypeCodePrefix 清理指定字典类型编码前缀的测试数据。
func cleanupSysDictByTypeCodePrefix(t *testing.T, typeCodePrefix string) {
	t.Helper()
	db, err := model.GetDB()
	if err != nil {
		return
	}

	var dictTypes []model.SysDictType
	if err := db.Unscoped().Where("type_code LIKE ?", typeCodePrefix+"%").Find(&dictTypes).Error; err != nil {
		return
	}
	if len(dictTypes) == 0 {
		return
	}

	typeIDs := make([]uint, 0, len(dictTypes))
	typeCodes := make([]string, 0, len(dictTypes))
	for _, dictType := range dictTypes {
		typeIDs = append(typeIDs, dictType.ID)
		typeCodes = append(typeCodes, dictType.TypeCode)
	}

	var itemIDs []uint
	_ = db.Unscoped().
		Model(&model.SysDictItem{}).
		Where("type_code IN ?", typeCodes).
		Pluck("id", &itemIDs).Error

	if len(itemIDs) > 0 {
		_ = db.Unscoped().Where("dict_item_id IN ?", itemIDs).Delete(&model.SysDictItemI18n{}).Error
	}
	_ = db.Unscoped().Where("type_code IN ?", typeCodes).Delete(&model.SysDictItem{}).Error
	_ = db.Unscoped().Where("dict_type_id IN ?", typeIDs).Delete(&model.SysDictTypeI18n{}).Error
	_ = db.Unscoped().Where("id IN ?", typeIDs).Delete(&model.SysDictType{}).Error
}
