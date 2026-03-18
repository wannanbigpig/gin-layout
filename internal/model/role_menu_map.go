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
