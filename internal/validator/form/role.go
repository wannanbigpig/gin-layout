package form

type RoleList struct {
	Paginate
	Status *int8  `form:"status" json:"status"  binding:"omitempty,oneof=0 1"`
	Name   string `form:"name" json:"name" binding:"omitempty"`
}

// NewRoleListQuery 初始化查询参数
func NewRoleListQuery() *RoleList {
	return &RoleList{}
}
