package errors

func init() {
	mergeZhCNText(map[int]string{
		SUCCESS:                   "OK",
		FAILURE:                   "FAIL",
		NotFound:                  "资源不存在",
		InvalidParameter:          "参数错误",
		ServerErr:                 "服务器内部错误",
		TooManyRequests:           "请求过多",
		UserDoesNotExist:          "用户不存在",
		UserDisable:               "用户已被禁用",
		ServiceDependencyNotReady: "服务暂未配置数据库，请联系管理员",
		NoExportData:              "暂无需要导出的数据",
		AuthorizationErr:          "暂无权限",
		NotLogin:                  "请先登录",
		CaptchaErr:                "验证码错误",
	})
}
