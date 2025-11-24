package model

// MenuApiMap 权限路由表
type MenuApiMap struct {
	BaseModel
	MenuId uint `json:"menu_id"` // 菜单ID
	ApiId  uint `json:"api_id"`  // API ID
}

func NewMenuApiMap() *MenuApiMap {
	return &MenuApiMap{}
}

// TableName 获取表名
func (m *MenuApiMap) TableName() string {
	return "a_menu_api_map"
}

func (m *MenuApiMap) BatchCreate(menuApi []*MenuApiMap) error {
	return m.DB().Create(&menuApi).Error
}
