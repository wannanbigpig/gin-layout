package form

type RoleList struct {
	Paginate
	Status *int8  `form:"status" json:"status"  binding:"omitempty,oneof=0 1"`
	Name   string `form:"name" json:"name" binding:"omitempty"`
	Code   string `form:"code" json:"code" binding:"omitempty,max=60"`
}

// NewRoleListQuery 初始化查询参数
func NewRoleListQuery() *RoleList {
	return &RoleList{}
}

// RoleOptions 角色选择器查询：按名称/编码模糊搜索，供下拉选择器使用。
type RoleOptions struct {
	Keyword string `form:"keyword" json:"keyword" binding:"omitempty,max=64"`
}

func NewRoleOptionsQuery() *RoleOptions {
	return &RoleOptions{}
}

type roleMutablePayload struct {
	Name        string `form:"name" json:"name" binding:"required"`
	Description string `form:"description" json:"description" binding:"omitempty"`
	Status      uint8  `form:"status" json:"status"  binding:"omitempty,oneof=0 1"`
	Sort        uint   `form:"sort" json:"sort" binding:"omitempty"`
	MenuList    []uint `form:"menu_list" json:"menu_list" binding:"omitempty,dive,gt=0"`
}

type CreateRole struct {
	Code string `form:"code" json:"code" binding:"omitempty,max=60"`
	roleMutablePayload
}

func NewCreateRoleForm() *CreateRole {
	return &CreateRole{}
}

type UpdateRole struct {
	Id                  uint    `form:"id" json:"id" binding:"required"`
	Code                *string `form:"code" json:"code" binding:"excluded_unless=ImmutableFieldGuard __never__"`
	ImmutableFieldGuard string  `form:"-" json:"-" binding:"-"`
	roleMutablePayload
}

func NewUpdateRoleForm() *UpdateRole {
	return &UpdateRole{}
}

func (f *UpdateRole) GetIDPointer() *uint {
	return &f.Id
}
