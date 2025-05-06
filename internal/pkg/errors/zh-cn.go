package errors

var zhCNText = map[int]string{
	SUCCESS:          "OK",
	FAILURE:          "FAIL",
	NotFound:         "资源不存在",
	InvalidParameter: "参数错误",
	ServerErr:        "服务器内部错误",
	TooManyRequests:  "请求过多",
	UserDoesNotExist: "用户不存在",
	UserDisable:      "用户已被禁用",
	AuthorizationErr: "暂无访问权限",
	NotLogin:         "请先登录",
	CaptchaErr:       "验证码错误",
}
