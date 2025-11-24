package form

type EditDept struct {
	Id          uint   `form:"id" json:"id" binding:"omitempty"`
	Name        string `form:"name" json:"name" label:"部门名称" binding:"required"`
	Pid         uint   `form:"pid" json:"pid" label:"上级部门" binding:"omitempty"`
	Description string `form:"description" json:"description" label:"描述" binding:"omitempty"`
	Sort        uint   `form:"sort" json:"sort" label:"排序" binding:"omitempty"`
}

func NewEditDeptForm() *EditDept {
	return &EditDept{}
}

type ListDept struct {
	Paginate
	Name string `form:"name" json:"name" label:"部门名称" binding:"omitempty"` // 关键字
	Pid  *uint  `form:"pid" json:"pid" label:"上级部门" binding:"omitempty"`
}

func NewDeptListQuery() *ListDept {
	return &ListDept{}
}
