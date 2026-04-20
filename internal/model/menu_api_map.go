package model

import "github.com/wannanbigpig/gin-layout/internal/global"

// MenuApiMap 权限路由表
type MenuApiMap struct {
	BaseModel
	MenuId uint `json:"menu_id"` // 菜单ID
	ApiId  uint `json:"api_id"`  // API ID
}

func NewMenuApiMap() *MenuApiMap {
	return BindModel(&MenuApiMap{})
}

// TableName 获取表名
func (m *MenuApiMap) TableName() string {
	return "menu_api_map"
}

func (m *MenuApiMap) CreateBatch(mappings []*MenuApiMap) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Create(&mappings).Error
}

// MenuIdsByApiIds 根据 API ID 列表查询关联的菜单 ID 列表。
func (m *MenuApiMap) MenuIdsByApiIds(apiIds []uint) ([]uint, error) {
	if len(apiIds) == 0 {
		return nil, nil
	}
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err := db.Where("api_id IN ?", apiIds).Pluck("menu_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// ApiPermission 接口权限信息（路由+方法）。
type ApiPermission struct {
	Route  string
	Method string
}

// ApiPermissionsByMenuIds 根据菜单 ID 列表查询去重后的接口权限（JOIN api 表）。
func (m *MenuApiMap) ApiPermissionsByMenuIds(menuIds []uint) ([]ApiPermission, error) {
	if len(menuIds) == 0 {
		return nil, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	var permissions []ApiPermission
	err = db.Table(m.TableName()+" m").
		Select("DISTINCT a.route, a.method").
		Joins("JOIN api a ON a.id = m.api_id").
		Where("m.menu_id IN ? AND a.deleted_at = 0 AND a.is_auth = ? AND a.is_effective = 1", menuIds, global.ApiAuthModeAuthz).
		Find(&permissions).Error
	if err != nil {
		return nil, err
	}
	return permissions, nil
}

// MenuApiPermission 按菜单分组的接口权限信息。
type MenuApiPermission struct {
	MenuId uint
	Route  string
	Method string
}

// MenuApiPermissionsByMenuIds 根据菜单 ID 列表查询按菜单分组的接口权限（JOIN api 表）。
func (m *MenuApiMap) MenuApiPermissionsByMenuIds(menuIds []uint) ([]MenuApiPermission, error) {
	if len(menuIds) == 0 {
		return nil, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	var rows []MenuApiPermission
	err = db.Table(m.TableName()+" m").
		Select("m.menu_id, a.route, a.method").
		Joins("JOIN api a ON a.id = m.api_id").
		Where("m.menu_id IN ? AND a.deleted_at = 0 AND a.is_auth = ? AND a.is_effective = 1", menuIds, global.ApiAuthModeAuthz).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
