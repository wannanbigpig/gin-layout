package autoload

import (
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

// AppConfig 定义应用运行时基础配置。
type AppConfig struct {
	// AppEnv 应用环境标识，如：local（本地）、dev（开发）、prod（生产）
	AppEnv string `mapstructure:"app_env"`
	// Debug 是否开启调试模式，true 时输出详细调试信息
	Debug bool `mapstructure:"debug"`
	// Language 国际化语言，如：zh_CN（中文）、en_US（英文）
	Language string `mapstructure:"language"`
	// WatchConfig 是否开启配置热更新，true 时配置变更自动重载
	WatchConfig bool `mapstructure:"watch_config"`
	// BasePath 应用基础路径，用于拼接文件存储路径。
	// 未在配置文件显式设置时，会在配置加载阶段优先回填为当前工作目录。
	BasePath string `mapstructure:"base_path"`
	// BaseURL 文件访问的基础 URL（如：https://example.com），用于拼接文件访问地址
	BaseURL string `mapstructure:"base_url"`
	// Timezone 时区设置，nil 时使用系统默认时区
	Timezone *string `mapstructure:"timezone"`
	// TrustedProxies 受信任代理列表，仅这些代理转发的 X-Forwarded-For/X-Real-IP 会被信任
	// 生产环境应配置为负载均衡或反向代理的 IP/网段
	TrustedProxies []string `mapstructure:"trusted_proxies"`
	// CorsOrigins CORS 允许的源列表，如：["http://localhost:3000", "https://example.com"]
	// 使用 ["*"] 表示允许所有源（生产环境慎用）
	CorsOrigins []string `mapstructure:"cors_origins"`
	// CorsMethods 允许的 HTTP 方法，如：["GET", "POST", "PUT", "DELETE"]
	// 使用 ["*"] 表示允许全部已支持方法，空数组使用默认值
	CorsMethods []string `mapstructure:"cors_methods"`
	// CorsHeaders 允许的请求头，如：["Content-Type", "Authorization"]
	// 使用 ["*"] 表示允许全部请求头
	CorsHeaders []string `mapstructure:"cors_headers"`
	// CorsExposeHeaders 暴露的响应头，如：["Content-Length", "X-Request-Id"]
	// 使用 ["*"] 表示暴露全部响应头
	CorsExposeHeaders []string `mapstructure:"cors_expose_headers"`
	// CorsMaxAge 预检请求（OPTIONS）缓存时间（秒），默认 43200（12 小时）
	CorsMaxAge int `mapstructure:"cors_max_age"`
	// CorsCredentials 是否允许携带凭证（cookies、Authorization 头等），默认 false
	CorsCredentials bool `mapstructure:"cors_credentials"`
	// AllowDegradedStartup 是否允许 service 在依赖初始化失败时降级启动。
	// true 时仅 HTTP 服务会继续启动，由 readiness 与路由守卫体现未就绪状态。
	AllowDegradedStartup bool `mapstructure:"allow_degraded_startup"`
	// EnableResetSystemCron 是否启用高风险的系统重建定时任务。
	// 默认 false，避免在非预期环境触发系统数据重建。
	EnableResetSystemCron bool `mapstructure:"enable_reset_system_cron"`
}

var App = AppConfig{
	AppEnv:                "local", // 默认本地环境
	Debug:                 true,    // 默认开启调试模式
	Language:              "zh_CN", // 默认中文
	WatchConfig:           false,   // 默认关闭配置热更新
	BasePath:              getDefaultPath(),
	BaseURL:               "",                    // 默认空，需要配置
	Timezone:              nil,                   // 默认使用系统时区
	TrustedProxies:        []string{"127.0.0.1"}, // 默认只信任本地
	CorsOrigins:           []string{},            // 默认空数组，不放行跨域来源；使用 ["*"] 表示允许所有源
	CorsMethods:           []string{},            // 默认空数组，使用默认方法列表；使用 ["*"] 表示允许全部已支持方法
	CorsHeaders:           []string{},            // 默认空数组，按请求头自动放行预检头；使用 ["*"] 表示允许全部请求头
	CorsExposeHeaders:     []string{},            // 默认空数组，默认暴露全部响应头；使用 ["*"] 明确表示暴露全部响应头
	CorsMaxAge:            43200,                 // 默认 12 小时（43200 秒）
	CorsCredentials:       false,                 // 默认不允许携带凭证
	AllowDegradedStartup:  false,                 // 默认关闭降级启动，依赖初始化失败时直接退出
	EnableResetSystemCron: false,                 // 默认关闭高风险系统重建定时任务
}

func getDefaultPath() (path string) {
	// 初始化时优先按 GO_ENV 处理：
	// - development: 当前工作目录
	// - 其他环境: 可执行文件所在目录
	// 配置加载阶段会按 app.base_path 是否显式配置进一步修正。
	path, err := utils.GetDefaultPath()
	if err != nil || path == "" {
		path = "."
	}
	return
}
