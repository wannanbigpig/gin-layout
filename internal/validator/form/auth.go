package form

type Login struct {
	UserName string `form:"username" json:"username"  binding:"required,min=5"` //  验证规则：必填，最小长度为5
	PassWord string `form:"password" json:"password"  binding:"required,min=6"` //  验证规则：必填，最小长度为6
}

func LoginForm() *Login {
	return &Login{}
}
