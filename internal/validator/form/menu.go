package form

type EditMenu struct {
	Id              uint    `form:"id" json:"id" binding:"omitempty"`
	Icon            string  `form:"icon" json:"icon" label:"图标" binding:"omitempty,max=255"`
	Title           string  `form:"title" json:"title" label:"标题" binding:"required,max=60"`
	Code            string  `form:"code" json:"code" label:"前端按钮权限标识" binding:"required_if=Type 3"`
	Path            string  `form:"path" json:"path" label:"路由地址" binding:"omitempty"`
	Name            string  `form:"name" json:"name" label:"前端路由名称" binding:"required_if_exist=Type 2"`
	AnimateEnter    string  `form:"animate_enter" json:"animate_enter" label:"进入动画，动画类参考URL_ADDRESS" binding:"omitempty"`
	AnimateLeave    string  `form:"animate_leave" json:"animate_leave" label:"离开动画，动画类参考URL_ADDRESS" binding:"omitempty"`
	AnimateDuration float32 `form:"animate_duration" json:"animate_duration" label:"动画持续时间" binding:"omitempty"`
	IsShow          int8    `form:"is_show" json:"is_show" label:"是否显示" binding:"omitempty,oneof=0 1"`               // 0 否 1 是
	IsAuth          int8    `form:"is_auth" json:"is_auth" label:"是否需要授权" binding:"omitempty,oneof=0 1"`           // 0 否 1 是
	IsNewWindow     int8    `form:"is_new_window" json:"is_new_window" label:"新窗口打开" binding:"omitempty,oneof=0 1"` // 0 否 1 是
	Sort            int32   `form:"sort" json:"sort" label:"排序" binding:"required"`
	Type            int8    `form:"type" json:"type" label:"菜单类型" binding:"required,oneof=1 2 3"` // 1 目录 2 菜单 3 按钮
	Pid             uint    `form:"pid" json:"pid" label:"上级菜单" binding:"omitempty"`
	Desc            string  `form:"desc" json:"desc" label:"描述" binding:"omitempty"`
	ApiList         []uint  `form:"api_list" json:"api_list" label:"接口列表" binding:"omitempty"`
	Component       string  `form:"component" json:"component" label:"前端组件路径"`
	Status          int8    `form:"status" json:"status" label:"状态" binding:"omitempty,oneof=0 1"` // 0 禁用 1 启用
	Redirect        string  `form:"redirect" json:"redirect" label:"重定向地址" binding:"omitempty"`
	IsExternalLinks int8    `form:"is_external_links" json:"is_external_links" label:"是否外链" binding:"omitempty,oneof=0 1"`
}

func NewEditMenuForm() *EditMenu {
	return &EditMenu{}
}

type ListMenu struct {
	Paginate
	Keyword string `form:"keyword" json:"keyword" binding:"omitempty"` // 关键字
	IsAuth  *int8  `form:"is_auth" json:"is_auth" binding:"omitempty"` // 是否授权
	Status  *int8  `form:"status" json:"status" binding:"omitempty"`   // 状态
}

func NewMenuListQuery() *ListMenu {
	return &ListMenu{}
}
