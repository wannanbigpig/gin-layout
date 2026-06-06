package model

import (
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model/modelDict"
)

// 角色状态字典
var RoleStatusDict modelDict.Dict = map[uint8]string{
	1: "启用",
	0: "禁用",
}

// Role 角色表
type Role struct {
	ContainsDeleteBaseModel
	Code        string        `json:"code" gorm:"column:code;type:varchar(60);not null;default:'';comment:角色业务编码"`
	IsSystem    uint8         `json:"is_system" gorm:"column:is_system;type:tinyint unsigned;not null;default:0;comment:是否系统保留对象"`
	Name        string        `json:"name" gorm:"column:name;type:varchar(60);not null;default:'';comment:角色名称"`
	Description string        `json:"description" gorm:"column:description;type:varchar(255);not null;default:'';comment:描述"`
	Sort        uint          `json:"sort" gorm:"column:sort;type:mediumint unsigned;not null;default:0;comment:排序"`
	MenuList    []RoleMenuMap `json:"menu_list,omitempty" gorm:"foreignkey:role_id;references:id;comment:菜单列表"`
	Status      uint8         `json:"status" gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:是否启用状态,1启用，0禁用"`
}

func NewRole() *Role {
	return BindModel(&Role{})
}

// TableName 获取表名
func (m *Role) TableName() string {
	return "role"
}

// StatusMap 状态映射
func (m *Role) StatusMap() string {
	return RoleStatusDict.Map(m.Status)
}

func (m *Role) IsSystemRole() bool {
	return m.IsSystem == global.Yes
}

// EnabledIdsByIds 根据 ID 列表查询启用状态（status=1）且未删除的角色 ID。
func (m *Role) EnabledIdsByIds(ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var result []uint
	if err := db.Where("id IN ? AND status = 1 AND deleted_at = 0", ids).Pluck("id", &result).Error; err != nil {
		return nil, err
	}
	return result, nil
}

// AllIds 查询所有未删除角色 ID。
func (m *Role) AllIds() ([]uint, error) {
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err := db.Where("deleted_at = 0").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// FindByCode 根据 code 查找未删除的角色，结果写入自身。
func (m *Role) FindByCode(code string) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where("code = ? AND deleted_at = 0", code).First(m).Error
}
