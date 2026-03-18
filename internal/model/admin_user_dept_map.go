package model

// AdminUserDeptMap 管理员用户部门关系表
type AdminUserDeptMap struct {
	BaseModel
	Uid    uint `json:"uid"`     // admin_user用户ID
	DeptId uint `json:"dept_id"` // 部门ID
}

func NewAdminUserDeptMap() *AdminUserDeptMap {
	return BindModel(&AdminUserDeptMap{})
}

// TableName 获取表名
func (m *AdminUserDeptMap) TableName() string {
	return "admin_user_department_map"
}

func (m *AdminUserDeptMap) CreateBatch(mappings []*AdminUserDeptMap) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Create(&mappings).Error
}
