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

// RoleIdsByUid 根据用户 ID 查询其关联的角色 ID 列表。
func (m *AdminUserRoleMap) RoleIdsByUid(uid uint) ([]uint, error) {
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err := db.Where("uid = ?", uid).Pluck("role_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// UidsByRoleIds 根据角色 ID 列表查询关联的用户 ID 列表。
func (m *AdminUserRoleMap) UidsByRoleIds(roleIds []uint) ([]uint, error) {
	if len(roleIds) == 0 {
		return nil, nil
	}
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err := db.Where("role_id IN ?", roleIds).Pluck("uid", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// UserRoleMapByUids 批量查询多个用户的角色关系，返回 map[uid][]roleId。
func (m *AdminUserRoleMap) UserRoleMapByUids(uids []uint) (map[uint][]uint, error) {
	result := make(map[uint][]uint, len(uids))
	if len(uids) == 0 {
		return result, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	type row struct {
		Uid    uint
		RoleId uint
	}
	var rows []row
	if err := db.Table(m.TableName()).Select("uid,role_id").Where("uid IN ?", uids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.Uid] = append(result[r.Uid], r.RoleId)
	}
	return result, nil
}

// CountByCondition 根据条件统计数量。
func (m *AdminUserRoleMap) CountByCondition(condition string, args ...any) (int64, error) {
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
func (m *AdminUserRoleMap) CreateOne() error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Create(m).Error
}
