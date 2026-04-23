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

type rolePayload struct {
	Code        string `form:"code" json:"code" binding:"omitempty,max=60"`
	Name        string `form:"name" json:"name" binding:"required"`
	Description string `form:"description" json:"description" binding:"omitempty"`
	Status      uint8  `form:"status" json:"status"  binding:"omitempty,oneof=0 1"`
	Pid         uint   `form:"pid" json:"pid" binding:"omitempty"`
	Sort        uint   `form:"sort" json:"sort" binding:"omitempty"`
	MenuList    []uint `form:"menu_ids" json:"menu_list" binding:"omitempty,dive,gt=0"`
}

type CreateRole struct {
	rolePayload
}

func NewCreateRoleForm() *CreateRole {
	return &CreateRole{}
}

type UpdateRole struct {
	Id uint `form:"id" json:"id" binding:"required"`
	rolePayload
}

func NewUpdateRoleForm() *UpdateRole {
	return &UpdateRole{}
}

func (f *UpdateRole) GetIDPointer() *uint {
	return &f.Id
}
