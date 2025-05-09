package model

// Role 角色表
type Role struct {
	ContainsDeleteBaseModel
	Pid         uint   `json:"pid" gorm:"column:pid;type:int unsigned;not null;default:0;comment:上级id"`
	Pids        string `json:"pids" gorm:"column:pids;type:varchar(255);not null;default:'';comment:所有上级id"`
	Name        string `json:"name" gorm:"column:name;type:varchar(60);not null;default:'';comment:角色名称"`
	Description string `json:"description" gorm:"column:description;type:varchar(255);not null;default:'';comment:描述"`
	Level       uint8  `json:"level" gorm:"column:level;type:tinyint unsigned;not null;default:1;comment:层级"`
	Sort        uint   `json:"sort" gorm:"column:sort;type:mediumint unsigned;not null;default:0;comment:排序"`
	Status      uint8  `json:"status" gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:是否启用状态,1启用，2不启用"`
}

func NewRole() *Role {
	return &Role{}
}

// TableName 获取表名
func (m *Role) TableName() string {
	return "a_role"
}
