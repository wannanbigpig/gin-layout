package errors

import "net/http"

// HTTP 状态码契约（与 response 层组合生效）：
//
//   - SUCCESS（code=0）           → 200
//   - 协议级失败（map 显式登记）   → 401/403/404/400/500/503
//   - 服务端故障（map 显式登记）   → 500
//   - 其余业务失败（未登记 code） → 400（fallback）
//
// 设计意图：
//   - 让网关 / CDN / APM 能直接基于 HTTP 状态码做错误率告警，业务失败不再隐藏在 200 之下；
//   - CDN/浏览器不会误缓存 4xx，避免业务异常被缓存污染；
//   - Chrome Network 标签页能直接定位失败请求，不再清一色绿色。
//
// 维护约定（新增业务错误码时遵循）：
//   - 客户端类失败（参数错、状态冲突、资源不存在、权限不足、业务规则不满足等）：**无需登记**，
//     fallback 会自动归为 400。
//   - 服务端类失败（依赖初始化、令牌生成、写库失败等技术性异常）：**必须登记到 500**，否则会
//     被错误地归为客户端问题，扭曲监控数据。
//   - 已映射的协议级 code（401/403/404/503 等）保持稳定，前端 `request.ts` 的 401
//     credential/session 分流契约依赖这部分映射，禁止删除。
var codeHTTPStatus = map[int]int{
	// ---- 协议级（与 HTTP 语义一一对应，跨业务稳定） ----
	NotLogin:                  http.StatusUnauthorized,        // 401 - Token 缺失/过期/会话失效
	FilePrivateAuthNeeded:     http.StatusUnauthorized,        // 401 - 私有文件访问需要有效登录态，供客户端续期重试
	AuthorizationErr:          http.StatusForbidden,           // 403 - 已登录但无权限
	NotFound:                  http.StatusNotFound,            // 404 - 资源不存在
	ServerErr:                 http.StatusInternalServerError, // 500 - 服务器内部错误
	ServiceDependencyNotReady: http.StatusServiceUnavailable,  // 503 - 依赖未就绪
	InvalidParameter:          http.StatusBadRequest,          // 400 - 参数校验失败（保留显式登记，语义最强）
	CaptchaErr:                http.StatusBadRequest,          // 400 - 验证码错误
	TooManyRequests:           http.StatusTooManyRequests,     // 429 - 业务级限流

	// ---- 登录凭证类（前端 401 credential 分流契约依赖，不可删） ----
	UserPasswordWrong:    http.StatusUnauthorized, // 401 - 登录凭证失败（对外泛化为“账号或密码错误”）
	UserDoesNotExist:     http.StatusUnauthorized, // 401 - 登录凭证失败（对外不暴露账号存在性）
	LoginAccountLocked:   http.StatusUnauthorized, // 401 - 登录账号被锁定
	RefreshTokenInvalid:  http.StatusUnauthorized, // 401 - Refresh Token 无效或已过期
	SMSCredentialInvalid: http.StatusUnauthorized, // 401 - 短信登录凭证失败
	SMSLoginDisabled:     http.StatusBadRequest,   // 400 - 短信登录未启用

	// ---- 服务端技术性故障（被 fallback 误归 4xx 会扭曲监控，必须显式 500） ----
	CasbinInitFailed:      http.StatusInternalServerError, // 500 - Casbin 初始化失败
	TokenGenerateFailed:   http.StatusInternalServerError, // 500 - JWT 令牌生成失败
	PasswordProcessFailed: http.StatusInternalServerError, // 500 - 密码加密/比对内部错误
	LoginFailed:           http.StatusInternalServerError, // 500 - 登录流程内部失败
	CreateUserFailed:      http.StatusInternalServerError, // 500 - 用户写库失败
	UpdateUserFailed:      http.StatusInternalServerError, // 500 - 用户写库失败
	DeleteUserFailed:      http.StatusInternalServerError, // 500 - 用户写库失败
	QueryUserDeptFailed:   http.StatusInternalServerError, // 500 - 用户部门查询失败
	StorageTestFailed:     http.StatusInternalServerError, // 500 - 存储供应商或网络测试失败
	SMSProviderFailed:     http.StatusBadGateway,          // 502 - 短信供应商调用失败
}

// InferHTTPStatus 根据业务错误码推断 HTTP 语义状态码。
//
//   - code == SUCCESS → 200；
//   - 命中显式映射 → 对应状态码；
//   - 其余（未登记的业务失败）→ 400。
//
// 把"业务失败默认 4xx"作为系统级契约，是为了让监控、缓存、调试三类外部观察者
// 直接从 HTTP 状态码读出系统健康度，而不再依赖业务方解析响应体。
func InferHTTPStatus(code int) int {
	if code == SUCCESS {
		return http.StatusOK
	}
	if status, ok := codeHTTPStatus[code]; ok {
		return status
	}
	return http.StatusBadRequest
}
