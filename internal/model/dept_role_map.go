package model

// DeptRoleMap 部门角色关联表
type DeptRoleMap struct {
	BaseModel
	DeptId uint `json:"dept_id"` // 菜单ID
	RoleId uint `json:"role_id"` // RoleID
}

func NewDeptRoleMap() *DeptRoleMap {
	return BindModel(&DeptRoleMap{})
}

// TableName 获取表名
func (m *DeptRoleMap) TableName() string {
	return "department_role_map"
}

func (m *DeptRoleMap) DeleteByDeptId(deptId uint) error {
	return m.DeleteWhere("dept_id = ?", deptId)
}

func (m *DeptRoleMap) CreateBatch(mappings []*DeptRoleMap) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Create(&mappings).Error
}
