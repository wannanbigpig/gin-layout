package model

import (
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model/modelDict"
)

// 角色状态字典
var RoleStatusDict modelDict.Dict = map[uint8]string{
	1: "启用",
	2: "禁用",
}

// Role 角色表
type Role struct {
	ContainsDeleteBaseModel
	Code        string        `json:"code" gorm:"column:code;type:varchar(60);not null;default:'';comment:角色业务编码"`
	IsSystem    uint8         `json:"is_system" gorm:"column:is_system;type:tinyint unsigned;not null;default:0;comment:是否系统保留对象"`
	Pid         uint          `json:"pid" gorm:"column:pid;type:int unsigned;not null;default:0;comment:上级id"`
	Pids        string        `json:"pids" gorm:"column:pids;type:varchar(255);not null;default:'';comment:所有上级id"`
	Name        string        `json:"name" gorm:"column:name;type:varchar(60);not null;default:'';comment:角色名称"`
	Description string        `json:"description" gorm:"column:description;type:varchar(255);not null;default:'';comment:描述"`
	Level       uint8         `json:"level" gorm:"column:level;type:tinyint unsigned;not null;default:1;comment:层级"`
	Sort        uint          `json:"sort" gorm:"column:sort;type:mediumint unsigned;not null;default:0;comment:排序"`
	ChildrenNum uint          `json:"children_num" gorm:"column:children_num;type:int unsigned;not null;default:0;comment:子集数量"`
	MenuList    []RoleMenuMap `json:"menu_list,omitempty" gorm:"foreignkey:role_id;references:id;comment:菜单列表"`
	Status      uint8         `json:"status" gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:是否启用状态,1启用，2不启用"`
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

// RoleStatusInfo 角色状态简要信息。
type RoleStatusInfo struct {
	ID     uint
	Pids   string
	Status uint8
}

// AllRoleStatusInfos 查询所有未删除角色的 id、pids、status 信息。
func (m *Role) AllRoleStatusInfos() ([]RoleStatusInfo, error) {
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	var rows []RoleStatusInfo
	if err := db.Table(m.TableName()).Select("id,pids,status").Where("deleted_at = 0").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// RoleTreeNode 角色树节点，用于展开子树。
type RoleTreeNode struct {
	ID   uint
	Pids string
}

// AllTreeNodes 查询所有未删除角色的 id、pids，用于角色子树展开。
func (m *Role) AllTreeNodes() ([]RoleTreeNode, error) {
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	var rows []RoleTreeNode
	if err := db.Table(m.TableName()).Select("id,pids").Where("deleted_at = 0").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
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

// FindByCode 根据 code 查找未删除的角色，结果写入自身。
func (m *Role) FindByCode(code string) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where("code = ? AND deleted_at = 0", code).First(m).Error
}

// FindPidsByIds 根据 ID 列表查询未删除角色的 id 和 pids 信息。
func (m *Role) FindPidsByIds(ids []uint) ([]RoleTreeNode, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	var rows []RoleTreeNode
	if err := db.Table(m.TableName()).Select("id,pids").Where("id IN ? AND deleted_at = 0", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// SubtreeIdsByRootIds 查询指定角色及其全部后代角色 ID。
func (m *Role) SubtreeIdsByRootIds(rootIDs []uint) ([]uint, error) {
	if len(rootIDs) == 0 {
		return nil, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}

	query := db.Table(m.TableName()).Where("deleted_at = 0").Where("id IN ?", rootIDs)
	for _, rootID := range rootIDs {
		query = query.Or("deleted_at = 0 AND FIND_IN_SET(?, pids)", rootID)
	}

	var ids []uint
	if err := query.Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// UpdateChildrenPidsByParent 批量更新指定父节点下所有子角色的 pids 和 level。
func (m *Role) UpdateChildrenPidsByParent(parentID uint, updateExpr string) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Model(m).
		Where("FIND_IN_SET(?,pids)", parentID).
		Updates(map[string]interface{}{
			"pids":  gorm.Expr(updateExpr),
			"level": gorm.Expr("length(pids) - length(replace(pids, ',', '')) + 1"),
		}).Error
}
