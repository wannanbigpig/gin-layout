package form

type menuPayload struct {
	Icon            string  `form:"icon" json:"icon" label:"图标" binding:"omitempty,max=255"`
	Title           string  `form:"title" json:"title" label:"标题" binding:"required,max=60"`
	Code            string  `form:"code" json:"code" label:"前端按钮权限标识" binding:"required_if=Type 3"`
	Path            string  `form:"path" json:"path" label:"路由地址" binding:"omitempty"`
	Name            string  `form:"name" json:"name" label:"前端路由名称" binding:"required_if_exist=Type 2"`
	AnimateEnter    string  `form:"animate_enter" json:"animate_enter" label:"进入动画，动画类参考URL_ADDRESS" binding:"omitempty"`
	AnimateLeave    string  `form:"animate_leave" json:"animate_leave" label:"离开动画，动画类参考URL_ADDRESS" binding:"omitempty"`
	AnimateDuration float32 `form:"animate_duration" json:"animate_duration" label:"动画持续时间" binding:"omitempty"`
	IsShow          uint8   `form:"is_show" json:"is_show" label:"是否显示" binding:"omitempty,oneof=0 1"`              // 0 否 1 是
	IsAuth          uint8   `form:"is_auth" json:"is_auth" label:"是否需要授权" binding:"omitempty,oneof=0 1"`            // 0 否 1 是
	IsNewWindow     uint8   `form:"is_new_window" json:"is_new_window" label:"新窗口打开" binding:"omitempty,oneof=0 1"` // 0 否 1 是
	Sort            uint    `form:"sort" json:"sort" label:"排序" binding:"required"`
	Type            uint8   `form:"type" json:"type" label:"菜单类型" binding:"required,oneof=1 2 3"` // 1 目录 2 菜单 3 按钮
	Pid             uint    `form:"pid" json:"pid" label:"上级菜单" binding:"omitempty"`
	Description     string  `form:"description" json:"description" label:"描述" binding:"omitempty"`
	ApiList         []uint  `form:"api_list" json:"api_list" label:"接口列表" binding:"omitempty"`
	Component       string  `form:"component" json:"component" label:"前端组件路径"`
	Status          uint8   `form:"status" json:"status" label:"状态" binding:"omitempty,oneof=0 1"` // 0 禁用 1 启用
	Redirect        string  `form:"redirect" json:"redirect" label:"重定向地址" binding:"omitempty"`
	IsExternalLinks uint8   `form:"is_external_links" json:"is_external_links" label:"是否外链" binding:"omitempty,oneof=0 1"`
}

type CreateMenu struct {
	menuPayload
}

func NewCreateMenuForm() *CreateMenu {
	return &CreateMenu{}
}

type UpdateMenu struct {
	Id uint `form:"id" json:"id" binding:"required"`
	menuPayload
}

func NewUpdateMenuForm() *UpdateMenu {
	return &UpdateMenu{}
}

func (f *UpdateMenu) GetIDPointer() *uint {
	return &f.Id
}

type ListMenu struct {
	Paginate
	Keyword string `form:"keyword" json:"keyword" binding:"omitempty"` // 关键字
	IsAuth  *int8  `form:"is_auth" json:"is_auth" binding:"omitempty"` // 是否授权
	Status  *int8  `form:"status" json:"status" binding:"omitempty"`   // 状态
}

// NewMenuListQuery 创建菜单列表查询表单。
func NewMenuListQuery() *ListMenu {
	return &ListMenu{}
}
