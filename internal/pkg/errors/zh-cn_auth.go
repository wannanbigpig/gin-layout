package errors

func init() {
	mergeZhCNText(map[int]string{
		UserPasswordWrong:       "用户密码错误",
		UserExists:              "用户已存在",
		PhoneNumberExists:       "手机号已存在",
		EmailExists:             "邮箱已存在",
		UsernameRequired:        "用户名必填",
		NicknameRequired:        "昵称必填",
		PasswordRequired:        "密码必填",
		PasswordProcessFailed:   "密码处理失败",
		SuperAdminCannotModify:  "系统默认超级管理员不允许修改密码",
		SuperAdminCannotDisable: "系统默认超级管理员不允许被禁用",
		SuperAdminCannotDelete:  "系统默认超级管理员不允许删除",
		SamePassword:            "新密码不能与当前密码相同",
		CasbinInitFailed:        "权限验证初始化失败",
		TokenGenerateFailed:     "生成Token失败",
		LoginFailed:             "登录失败，请稍后重试",
		CreateUserFailed:        "创建用户失败，请重试",
		UpdateUserFailed:        "更新用户失败，请重试",
		DeleteUserFailed:        "删除用户失败，请重试",
		QueryUserDeptFailed:     "查询用户部门关联失败",
		SuperAdminMustKeepRole:  "系统默认超级管理员必须保留超级管理员角色",
		LoginAccountLocked:      "账号已被锁定，请稍后重试",
		RefreshTokenInvalid:     "刷新令牌无效或已过期",
	})

	mergeZhCNTextKey(map[string]string{
		MsgKeyAuthSessionExpired:        "登录已失效，请重新登录",
		MsgKeyAuthPermissionInitFailed:  "权限验证初始化失败",
		MsgKeyAuthPermissionCheckFailed: "权限验证失败",
		MsgKeyAuthAPIOperationDenied:    "暂无接口操作权限",
		MsgKeyAuthAccountLocked:         "账号已被锁定，请在 %d 分钟后重试",
	})
}
