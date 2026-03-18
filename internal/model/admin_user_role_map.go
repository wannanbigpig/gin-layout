package model

// AdminUserRoleMap 管理员用户角色关系表
type AdminUserRoleMap struct {
	BaseModel
	Uid    uint `json:"uid"`     // admin_user用户ID
	RoleId uint `json:"role_id"` // RoleID
}

func NewAdminUserRoleMap() *AdminUserRoleMap {
	return BindModel(&AdminUserRoleMap{})
}

// TableName 获取表名
func (m *AdminUserRoleMap) TableName() string {
	return "admin_user_role_map"
}

func (m *AdminUserRoleMap) CreateBatch(mappings []*AdminUserRoleMap) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Create(&mappings).Error
}
