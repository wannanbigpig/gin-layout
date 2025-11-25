package form

type RoleList struct {
	Paginate
	Status *int8  `form:"status" json:"status"  binding:"omitempty,oneof=0 1"`
	Name   string `form:"name" json:"name" binding:"omitempty"`
	Pid    *uint  `form:"pid" json:"pid" binding:"omitempty"`
}

// NewRoleListQuery 初始化查询参数
func NewRoleListQuery() *RoleList {
	return &RoleList{}
}

type EditRole struct {
	Id          uint   `form:"id" json:"id" binding:"omitempty"`
	Name        string `form:"name" json:"name" binding:"required"`
	Description string `form:"description" json:"description" binding:"omitempty"`
	Status      uint8  `form:"status" json:"status"  binding:"omitempty,oneof=0 1"`
	Pid         uint   `form:"pid" json:"pid" binding:"omitempty"`
	Sort        uint   `form:"sort" json:"sort" binding:"omitempty"`
	MenuList    []uint `form:"menu_ids" json:"menu_list" binding:"omitempty"`
}

func NewEditRoleForm() *EditRole {
	return &EditRole{}
}
