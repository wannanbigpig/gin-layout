package global

// ApiAuthMode 定义 API 路由的鉴权模式。
type ApiAuthMode uint8

const (
	// ApiAuthModeNone 无需登录，无需 API 权限校验。
	ApiAuthModeNone ApiAuthMode = iota
	// ApiAuthModeLogin 需要登录，但无需 API 权限校验。
	ApiAuthModeLogin
	// ApiAuthModeAuthz 需要登录且需要 API 权限校验。
	ApiAuthModeAuthz
)

// RequiresLogin 返回该模式是否要求用户先登录。
func (m ApiAuthMode) RequiresLogin() bool {
	return m != ApiAuthModeNone
}

// RequiresAPIPermission 返回该模式是否要求 API 权限校验。
func (m ApiAuthMode) RequiresAPIPermission() bool {
	return m == ApiAuthModeAuthz
}

// Label 返回该模式的人类可读名称。
func (m ApiAuthMode) Label() string {
	switch m {
	case ApiAuthModeNone:
		return "无需登录"
	case ApiAuthModeLogin:
		return "需要登录"
	case ApiAuthModeAuthz:
		return "需要登录和API权限"
	default:
		return "-"
	}
}
