package form

type Add struct {
	UserName string `form:"username" json:"username"  binding:"required,min=5"` //  验证规则：必填，最小长度为5
	PassWord string `form:"password" json:"password"  binding:"required,min=6"` //  验证规则：必填，最小长度为6
	Mobile   string `form:"mobile" json:"mobile"  binding:"omitempty,mobile"`
	Email    string `form:"email" json:"email"  binding:"omitempty,email"`
	IsAdmin  int8   `form:"is_admin" json:"is_admin"  binding:"omitempty,oneof=0 1"`
}
