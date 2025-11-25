package model

// Department 部门表
type Department struct {
	ContainsDeleteBaseModel
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
	return &Department{}
}

// TableName 获取表名
func (m *Department) TableName() string {
	return "a_department"
}
