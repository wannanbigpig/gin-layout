package model

import (
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
)

// Department 部门表
type Department struct {
	ContainsDeleteBaseModel
	Code        string        `json:"code" gorm:"column:code;type:varchar(60);not null;default:'';comment:部门业务编码"`
	IsSystem    uint8         `json:"is_system" gorm:"column:is_system;type:tinyint unsigned;not null;default:0;comment:是否系统保留对象"`
	Pid         uint          `json:"pid" gorm:"column:pid;type:int unsigned;not null;default:0;comment:上级id"`
	Pids        string        `json:"pids" gorm:"column:pids;type:varchar(255);not null;default:'';comment:所有上级id"`
	Name        string        `json:"name" gorm:"column:name;type:varchar(60);not null;default:'';comment:部门名称"`
	Description string        `json:"description" gorm:"column:description;type:varchar(255);not null;default:'';comment:描述"`
	Level       uint8         `json:"level" gorm:"column:level;type:tinyint unsigned;not null;default:1;comment:层级"`
	Sort        uint          `json:"sort" gorm:"column:sort;type:mediumint unsigned;not null;default:0;comment:排序"`
	ChildrenNum uint          `json:"children_num" gorm:"column:children_num;type:int unsigned;not null;default:0;comment:子集数量"`
	UserNumber  uint          `json:"user_number" gorm:"column:user_number;type:int unsigned;not null;default:0;comment:用户数量"`
	RoleList    []DeptRoleMap `json:"role_list" gorm:"foreignKey:dept_id;references:id"`
}

func NewDepartment() *Department {
	return BindModel(&Department{})
}

// TableName 获取表名
func (m *Department) TableName() string {
	return "department"
}

func (m *Department) IsSystemDepartment() bool {
	return m.IsSystem == global.Yes
}

// FindByCode 根据 code 查找未删除的部门，结果写入自身。
func (m *Department) FindByCode(code string) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where("code = ? AND deleted_at = 0", code).First(m).Error
}

// UpdateUserNumberByIds 批量更新指定部门的用户数量。
func (m *Department) UpdateUserNumberByIds(deptIds []uint, updateExpr string) error {
	if len(deptIds) == 0 {
		return nil
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Model(m).
		Where("id IN (?)", deptIds).
		Update("user_number", gorm.Expr(updateExpr)).Error
}

// UpdateChildrenPidsByParent 批量更新指定父节点下所有子部门的 pids 和 level。
func (m *Department) UpdateChildrenPidsByParent(parentID uint, updateExpr string) error {
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
