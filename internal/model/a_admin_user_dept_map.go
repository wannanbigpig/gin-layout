package model

// AdminUsesDeptMap 管理员用户部门关系表
type AdminUsesDeptMap struct {
	BaseModel
	Uid    uint `json:"uid"`     // admin_user用户ID
	DeptId uint `json:"dept_id"` // RoleID
}

func NewAdminUsesDeptMap() *AdminUsesDeptMap {
	return &AdminUsesDeptMap{}
}

// TableName 获取表名
func (m *AdminUsesDeptMap) TableName() string {
	return "a_admin_user_department_map"
}

func (m *AdminUsesDeptMap) BatchCreate(dept []*AdminUsesDeptMap) error {
	return m.DB().Create(&dept).Error
}
