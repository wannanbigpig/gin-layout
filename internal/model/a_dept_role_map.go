package model

// DeptRoleMap 部门角色关联表
type DeptRoleMap struct {
	BaseModel
	DeptId uint `json:"dept_id"` // 菜单ID
	RoleId uint `json:"role_id"` // RoleID
}

func NewDeptRoleMap() *DeptRoleMap {
	return &DeptRoleMap{}
}

// TableName 获取表名
func (m *DeptRoleMap) TableName() string {
	return "a_department_role_map"
}

func (m *DeptRoleMap) DeleteByDeptId(deptId uint) error {
	return m.DB().Where("dept_id = ?", deptId).Delete(&DeptRoleMap{}).Error
}

func (m *DeptRoleMap) BatchCreate(deptRole []*DeptRoleMap) error {
	return m.DB().Create(&deptRole).Error
}
