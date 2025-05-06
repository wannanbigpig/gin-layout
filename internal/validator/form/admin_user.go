package form

type EditAdminUser struct {
	Id          uint   `form:"id" json:"id" label:"用户ID" binding:"omitempty"`                                                //  验证规则：必填，最小长度为5
	Username    string `form:"username" json:"username" label:"用户名" binding:"required,min=3,max=20,regexp=^[a-zA-Z0-9_]+$"` //  验证规则：必填，只能是字母、数字、下划线，至少3位
	Nickname    string `form:"nickname" json:"nickname" label:"昵称" binding:"required"`
	Password    string `form:"password" json:"password" label:"密码" binding:"omitempty,min=6,max=32"` //  验证规则：必填，最小长度为6
	PhoneNumber string `form:"phone_number" json:"phone_number" label:"手机号" binding:"omitempty,phone_number"`
	CountryCode string `form:"country_code" json:"country_code" label:"手机号" binding:"omitempty"`
	Email       string `form:"email" json:"email" label:"邮箱" binding:"omitempty,email"`
	Status      int8   `form:"status" json:"status" label:"状态"  binding:"omitempty,oneof=0 1"`
	Avatar      string `form:"avatar" json:"avatar" label:"头像" binding:"omitempty"`
}

// NewEditAdminUser 创建一个新的管理员用户编辑器
func NewEditAdminUser() *EditAdminUser {
	return &EditAdminUser{}
}

type AdminUserList struct {
	Paginate
	Email       string `form:"email" json:"email" binding:"omitempty,email"`
	UserName    string `form:"username" json:"username" binding:"omitempty"`
	Status      *int8  `form:"status" json:"status"  binding:"omitempty,oneof=0 1"`
	PhoneNumber string `form:"phone_number" json:"phone_number" binding:"omitempty,phone_number"`
	NickName    string `form:"nickname" json:"nickname" binding:"omitempty"`
	ID          uint   `form:"id" json:"id" binding:"omitempty"`
}

// NewAdminUserListQuery 创建一个新的管理员用户列表查询
func NewAdminUserListQuery() *AdminUserList {
	return &AdminUserList{}
}
