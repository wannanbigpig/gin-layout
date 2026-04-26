package model

// SysDictType 系统字典类型表。
type SysDictType struct {
	ContainsDeleteBaseModel
	TypeCode     string            `json:"type_code" gorm:"column:type_code;type:varchar(100);not null;default:'';comment:字典类型编码"`
	TypeName     string            `json:"type_name" gorm:"-:all"`
	TypeNameI18n map[string]string `json:"type_name_i18n" gorm:"-:all"`
	IsSystem     uint8             `json:"is_system" gorm:"column:is_system;type:tinyint unsigned;not null;default:0;comment:是否系统内置"`
	Status       uint8             `json:"status" gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:状态"`
	Sort         uint              `json:"sort" gorm:"column:sort;type:int unsigned;not null;default:0;comment:排序"`
	Remark       string            `json:"remark" gorm:"column:remark;type:varchar(255);not null;default:'';comment:备注"`
}

// SysDictItem 系统字典项表。
type SysDictItem struct {
	ContainsDeleteBaseModel
	TypeCode  string            `json:"type_code" gorm:"column:type_code;type:varchar(100);not null;default:'';comment:字典类型编码"`
	Label     string            `json:"label" gorm:"-:all"`
	LabelI18n map[string]string `json:"label_i18n" gorm:"-:all"`
	Value     string            `json:"value" gorm:"column:value;type:varchar(100);not null;default:'';comment:字典值"`
	Color     string            `json:"color" gorm:"column:color;type:varchar(30);not null;default:'';comment:展示颜色"`
	TagType   string            `json:"tag_type" gorm:"column:tag_type;type:varchar(30);not null;default:'';comment:前端标签类型"`
	IsDefault uint8             `json:"is_default" gorm:"column:is_default;type:tinyint unsigned;not null;default:0;comment:是否默认项"`
	IsSystem  uint8             `json:"is_system" gorm:"column:is_system;type:tinyint unsigned;not null;default:0;comment:是否系统内置"`
	Status    uint8             `json:"status" gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:状态"`
	Sort      uint              `json:"sort" gorm:"column:sort;type:int unsigned;not null;default:0;comment:排序"`
	Remark    string            `json:"remark" gorm:"column:remark;type:varchar(255);not null;default:'';comment:备注"`
}

func NewSysDictType() *SysDictType {
	return BindModel(&SysDictType{})
}

func NewSysDictItem() *SysDictItem {
	return BindModel(&SysDictItem{})
}

func (m *SysDictType) TableName() string {
	return "sys_dict_type"
}

func (m *SysDictItem) TableName() string {
	return "sys_dict_item"
}

// IsProtected 判断字典类型是否为系统内置保护项。
func (m *SysDictType) IsProtected() bool {
	return m != nil && m.IsSystem == 1
}

// IsProtected 判断字典项是否为系统内置保护项。
func (m *SysDictItem) IsProtected() bool {
	return m != nil && m.IsSystem == 1
}

// FindByTypeCode 根据类型编码查询未删除字典类型。
func (m *SysDictType) FindByTypeCode(typeCode string) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where("type_code = ? AND deleted_at = 0", typeCode).First(m).Error
}

// ExistsByTypeCodeExcludeID 检查类型编码是否已被其他记录占用。
func (m *SysDictType) ExistsByTypeCodeExcludeID(typeCode string, excludeID uint) (bool, error) {
	db, err := m.GetDB()
	if err != nil {
		return false, err
	}
	var count int64
	query := db.Model(m).Where("type_code = ? AND deleted_at = 0", typeCode)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ExistsByValueExcludeID 检查同类型下字典值是否已被其他记录占用。
func (m *SysDictItem) ExistsByValueExcludeID(typeCode, value string, excludeID uint) (bool, error) {
	db, err := m.GetDB()
	if err != nil {
		return false, err
	}
	var count int64
	query := db.Model(m).Where("type_code = ? AND value = ? AND deleted_at = 0", typeCode, value)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindByTypeCodeAndValue 根据类型编码和字典值查询未删除字典项。
func (m *SysDictItem) FindByTypeCodeAndValue(typeCode, value string) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where("type_code = ? AND value = ? AND deleted_at = 0", typeCode, value).First(m).Error
}

// EnabledItemsByTypeCode 查询指定类型下启用字典项。
func (m *SysDictItem) EnabledItemsByTypeCode(typeCode string) ([]SysDictItem, error) {
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	var items []SysDictItem
	err = db.Where("type_code = ? AND status = 1 AND deleted_at = 0", typeCode).
		Order("sort desc, id asc").
		Find(&items).Error
	return items, err
}

// CountByTypeCode 统计指定类型下未删除字典项数量。
func (m *SysDictItem) CountByTypeCode(typeCode string) (int64, error) {
	db, err := m.GetDB()
	if err != nil {
		return 0, err
	}
	var count int64
	err = db.Model(m).Where("type_code = ? AND deleted_at = 0", typeCode).Count(&count).Error
	return count, err
}
