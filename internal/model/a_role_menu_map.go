package model

// RoleMenuMap 角色菜单关联表
type RoleMenuMap struct {
	BaseModel
	MenuId uint `json:"menu_id"` // 菜单ID
	RoleId uint `json:"role_id"` // RoleID
}

func NewRoleMenuMap() *RoleMenuMap {
	return &RoleMenuMap{}
}

// TableName 获取表名
func (m *RoleMenuMap) TableName() string {
	return "a_role_menu_map"
}

func (m *RoleMenuMap) DeleteByMenuId(roleId uint) error {
	return m.DB().Where("role_id = ?", roleId).Delete(&RoleMenuMap{}).Error
}

func (m *RoleMenuMap) BatchCreate(roleMenu []*RoleMenuMap) error {
	return m.DB().Create(&roleMenu).Error
}
