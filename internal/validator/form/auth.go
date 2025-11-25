package form

type LoginAuth struct {
	UserName  string `form:"username" json:"username" label:"用户名" binding:"required,min=3,max=16"` //  验证规则：必填，最小长度为3
	PassWord  string `form:"password" json:"password" label:"密码" binding:"required,min=6,max=18"`  //  验证规则：必填，最小长度为6
	Captcha   string `form:"captcha" json:"captcha" label:"验证码" binding:"required"`
	CaptchaID string `form:"captcha_id" json:"captcha_id" binding:"required"`
}

func NewLoginForm() *LoginAuth {
	return &LoginAuth{}
}
