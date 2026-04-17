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

// RoleIdsByDeptIds 根据部门 ID 列表查询关联的角色 ID 列表。
func (m *DeptRoleMap) RoleIdsByDeptIds(deptIds []uint) ([]uint, error) {
	if len(deptIds) == 0 {
		return nil, nil
	}
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err := db.Where("dept_id IN ?", deptIds).Pluck("role_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// DeptIdsByRoleIds 根据角色 ID 列表查询关联的部门 ID 列表。
func (m *DeptRoleMap) DeptIdsByRoleIds(roleIds []uint) ([]uint, error) {
	if len(roleIds) == 0 {
		return nil, nil
	}
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err := db.Where("role_id IN ?", roleIds).Pluck("dept_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// DeptRoleMapByDeptIds 批量查询多个部门的角色关系，返回 map[deptId][]roleId。
func (m *DeptRoleMap) DeptRoleMapByDeptIds(deptIds []uint) (map[uint][]uint, error) {
	result := make(map[uint][]uint, len(deptIds))
	if len(deptIds) == 0 {
		return result, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	type row struct {
		DeptId uint
		RoleId uint
	}
	var rows []row
	if err := db.Table(m.TableName()).Select("dept_id,role_id").Where("dept_id IN ?", deptIds).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.DeptId] = append(result[r.DeptId], r.RoleId)
	}
	return result, nil
}
