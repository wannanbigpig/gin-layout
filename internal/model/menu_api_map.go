package model

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
