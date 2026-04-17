package model

// RoleMenuMap 角色菜单关联表
type RoleMenuMap struct {
	BaseModel
	MenuId uint `json:"menu_id"` // 菜单ID
	RoleId uint `json:"role_id"` // RoleID
}

func NewRoleMenuMap() *RoleMenuMap {
	return BindModel(&RoleMenuMap{})
}

// TableName 获取表名
func (m *RoleMenuMap) TableName() string {
	return "role_menu_map"
}

func (m *RoleMenuMap) CreateBatch(mappings []*RoleMenuMap) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Create(&mappings).Error
}

// MenuIdsByRoleIds 根据角色 ID 列表查询关联的菜单 ID 列表。
func (m *RoleMenuMap) MenuIdsByRoleIds(roleIds []uint) ([]uint, error) {
	if len(roleIds) == 0 {
		return nil, nil
	}
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err := db.Where("role_id IN ?", roleIds).Pluck("menu_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// RoleIdsByMenuIds 根据菜单 ID 列表查询关联的角色 ID 列表。
func (m *RoleMenuMap) RoleIdsByMenuIds(menuIds []uint) ([]uint, error) {
	if len(menuIds) == 0 {
		return nil, nil
	}
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err := db.Where("menu_id IN ?", menuIds).Pluck("role_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// RoleMenuMapByRoleIds 批量查询多个角色的菜单关系，返回 map[roleId][]menuId。
func (m *RoleMenuMap) RoleMenuMapByRoleIds(roleIds []uint) (map[uint][]uint, error) {
	result := make(map[uint][]uint)
	if len(roleIds) == 0 {
		return result, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	type row struct {
		RoleId uint
		MenuId uint
	}
	var rows []row
	if err := db.Table(m.TableName()).Select("role_id,menu_id").Where("role_id IN ?", roleIds).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.RoleId] = append(result[r.RoleId], r.MenuId)
	}
	return result, nil
}
