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

// DeptIdsByUid 根据用户 ID 查询其关联的部门 ID 列表。
func (m *AdminUserDeptMap) DeptIdsByUid(uid uint) ([]uint, error) {
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err := db.Where("uid = ?", uid).Pluck("dept_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// UidsByDeptIds 根据部门 ID 列表查询关联的用户 ID 列表。
func (m *AdminUserDeptMap) UidsByDeptIds(deptIds []uint) ([]uint, error) {
	if len(deptIds) == 0 {
		return nil, nil
	}
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err := db.Where("dept_id IN ?", deptIds).Pluck("uid", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// UserDeptMapByUids 批量查询多个用户的部门关系，返回 map[uid][]deptId。
func (m *AdminUserDeptMap) UserDeptMapByUids(uids []uint) (map[uint][]uint, []uint, error) {
	result := make(map[uint][]uint, len(uids))
	if len(uids) == 0 {
		return result, nil, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, nil, err
	}
	type row struct {
		Uid    uint
		DeptId uint
	}
	var rows []row
	if err := db.Table(m.TableName()).Select("uid,dept_id").Where("uid IN ?", uids).Scan(&rows).Error; err != nil {
		return nil, nil, err
	}
	deptIds := make([]uint, 0, len(rows))
	for _, r := range rows {
		result[r.Uid] = append(result[r.Uid], r.DeptId)
		deptIds = append(deptIds, r.DeptId)
	}
	return result, deptIds, nil
}

// CountByCondition 根据条件统计数量。
func (m *AdminUserDeptMap) CountByCondition(condition string, args ...any) (int64, error) {
	db, err := m.GetDB(m)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := db.Where(condition, args...).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CreateOne 创建单条记录。
func (m *AdminUserDeptMap) CreateOne() error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Create(m).Error
}
