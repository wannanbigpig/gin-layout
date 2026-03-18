package form

type adminUserEditableFields struct {
	Username    *string `form:"username" json:"username" label:"用户名" binding:"omitempty,min=3,max=20,regexp=^[a-zA-Z0-9_]+$"`
	Nickname    *string `form:"nickname" json:"nickname" label:"昵称" binding:"omitempty"`
	Password    *string `form:"password" json:"password" label:"密码" binding:"omitempty,min=6,max=32"`
	PhoneNumber *string `form:"phone_number" json:"phone_number" label:"手机号" binding:"omitempty,phone_number"`
	CountryCode *string `form:"country_code" json:"country_code" label:"国家代码" binding:"omitempty"`
	Email       *string `form:"email" json:"email" label:"邮箱" binding:"omitempty,email"`
	Status      *uint8  `form:"status" json:"status" label:"状态" binding:"omitempty,oneof=0 1"`
	Avatar      *string `form:"avatar" json:"avatar" label:"头像" binding:"omitempty"`
	DeptIds     []uint  `form:"dept_ids" json:"dept_ids" label:"部门ID" binding:"omitempty"`
}

type CreateAdminUser struct {
	adminUserEditableFields
}

func NewCreateAdminUser() *CreateAdminUser {
	return &CreateAdminUser{}
}

type UpdateAdminUser struct {
	Id uint `form:"id" json:"id" label:"用户ID" binding:"required"`
	adminUserEditableFields
}

func NewUpdateAdminUser() *UpdateAdminUser {
	return &UpdateAdminUser{}
}

type AdminUserList struct {
	Paginate
	Email       string `form:"email" json:"email" binding:"omitempty,email"`
	UserName    string `form:"username" json:"username" binding:"omitempty"`
	Status      *uint8 `form:"status" json:"status"  binding:"omitempty,oneof=0 1"`
	PhoneNumber string `form:"phone_number" json:"phone_number" binding:"omitempty,phone_number"`
	NickName    string `form:"nickname" json:"nickname" binding:"omitempty"`
	ID          uint   `form:"id" json:"id" binding:"omitempty"`
	DeptId      uint   `form:"dept_id" json:"dept_id" binding:"omitempty"`
}

func NewAdminUserListQuery() *AdminUserList {
	return &AdminUserList{}
}

type UpdateProfile struct {
	Nickname    *string `form:"nickname" json:"nickname" label:"昵称" binding:"omitempty"`
	Password    *string `form:"password" json:"password" label:"密码" binding:"omitempty,min=6,max=32"`
	PhoneNumber *string `form:"phone_number" json:"phone_number" label:"手机号" binding:"omitempty,phone_number"`
	CountryCode *string `form:"country_code" json:"country_code" label:"国家代码" binding:"omitempty"`
	Email       *string `form:"email" json:"email" label:"邮箱" binding:"omitempty,email"`
	Avatar      *string `form:"avatar" json:"avatar" label:"头像" binding:"omitempty"`
}

func NewUpdateProfile() *UpdateProfile {
	return &UpdateProfile{}
}
