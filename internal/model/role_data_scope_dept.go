package model

// RoleDataScopeDept 角色数据权限部门关联表
type RoleDataScopeDept struct {
	BaseModel
	RoleId uint `json:"role_id" gorm:"column:role_id;type:int unsigned;not null;default:0;comment:角色ID"`
	DeptId uint `json:"dept_id" gorm:"column:dept_id;type:int unsigned;not null;default:0;comment:部门ID"`
}

func NewRoleDataScopeDept() *RoleDataScopeDept {
	return BindModel(&RoleDataScopeDept{})
}

// TableName 获取表名
func (m *RoleDataScopeDept) TableName() string {
	return "role_data_scope_dept"
}

func (m *RoleDataScopeDept) CreateBatch(mappings []*RoleDataScopeDept) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Create(&mappings).Error
}

// DeptIdsByRoleId 根据角色 ID 查询其关联的部门 ID 列表。
func (m *RoleDataScopeDept) DeptIdsByRoleId(roleId uint) ([]uint, error) {
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err := db.Where("role_id = ?", roleId).Pluck("dept_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// DeptIdsByRoleIds 根据角色 ID 列表查询关联的部门 ID 列表（去重）。
func (m *RoleDataScopeDept) DeptIdsByRoleIds(roleIds []uint) ([]uint, error) {
	if len(roleIds) == 0 {
		return nil, nil
	}
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err := db.Where("role_id IN ?", roleIds).Distinct().Pluck("dept_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// DeleteByRoleId 删除指定角色的所有自定义部门关联。
func (m *RoleDataScopeDept) DeleteByRoleId(roleId uint) error {
	return m.DeleteWhere("role_id = ?", roleId)
}
