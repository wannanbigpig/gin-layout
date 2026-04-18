package form

type deptPayload struct {
	Name        string `form:"name" json:"name" label:"部门名称" binding:"required"`
	Pid         uint   `form:"pid" json:"pid" label:"上级部门" binding:"omitempty"`
	Description string `form:"description" json:"description" label:"描述" binding:"omitempty"`
	Sort        uint   `form:"sort" json:"sort" label:"排序" binding:"omitempty"`
}

type CreateDept struct {
	deptPayload
}

func NewCreateDeptForm() *CreateDept {
	return &CreateDept{}
}

type UpdateDept struct {
	Id uint `form:"id" json:"id" binding:"required"`
	deptPayload
}

func NewUpdateDeptForm() *UpdateDept {
	return &UpdateDept{}
}

func (f *UpdateDept) GetIDPointer() *uint {
	return &f.Id
}

type ListDept struct {
	Paginate
	Name string `form:"name" json:"name" label:"部门名称" binding:"omitempty"` // 关键字
	Pid  *uint  `form:"pid" json:"pid" label:"上级部门" binding:"omitempty"`
}

// NewDeptListQuery 创建部门列表查询表单。
func NewDeptListQuery() *ListDept {
	return &ListDept{}
}

// DeptBindRole 部门绑定角色表单
type DeptBindRole struct {
	DeptId  uint   `form:"dept_id" json:"dept_id" label:"部门 ID" binding:"required"`   //  验证规则：必填
	RoleIds []uint `form:"role_ids" json:"role_ids" label:"角色 ID" binding:"required"` //  验证规则：必填
}

// NewDeptBindRole 部门绑定角色
func NewDeptBindRole() *DeptBindRole {
	return &DeptBindRole{}
}
