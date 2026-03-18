package autoload

import (
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

// AppConfig 定义应用运行时基础配置。
type AppConfig struct {
	AppEnv      string  `mapstructure:"app_env"`
	Debug       bool    `mapstructure:"debug"`
	Language    string  `mapstructure:"language"`
	WatchConfig bool    `mapstructure:"watch_config"`
	BasePath    string  `mapstructure:"base_path"`
	BaseURL     string  `mapstructure:"base_url"` // 文件访问的基础URL（如：https://example.com）
	Timezone    *string `mapstructure:"timezone"`
	// 受信任代理配置
	TrustedProxies []string `mapstructure:"trusted_proxies"` // 允许解析 X-Forwarded-For/X-Real-IP 的代理地址或网段
	// CORS 配置
	CorsOrigins       []string `mapstructure:"cors_origins"`        // CORS允许的源列表（如：["http://localhost:3000", "https://example.com"]），空数组表示允许所有源
	CorsMethods       []string `mapstructure:"cors_methods"`        // 允许的HTTP方法（如：["GET", "POST", "PUT", "DELETE"]），空数组使用默认值
	CorsHeaders       []string `mapstructure:"cors_headers"`        // 允许的请求头（如：["Content-Type", "Authorization"]），空数组表示允许所有（使用 "*"）
	CorsExposeHeaders []string `mapstructure:"cors_expose_headers"` // 暴露的响应头（如：["Content-Length", "X-Request-Id"]），空数组表示暴露所有（使用 "*"）
	CorsMaxAge        int      `mapstructure:"cors_max_age"`        // 预检请求缓存时间（秒），默认 43200（12小时）
	CorsCredentials   bool     `mapstructure:"cors_credentials"`    // 是否允许携带凭证（cookies等），默认 false
}

var App = AppConfig{
	AppEnv:            "local",
	Debug:             true,
	Language:          "zh_CN",
	WatchConfig:       false,
	BasePath:          getDefaultPath(),
	BaseURL:           "", // 默认空，需要配置
	Timezone:          nil,
	TrustedProxies:    []string{"127.0.0.1"},
	CorsOrigins:       []string{}, // 默认空数组，表示允许所有源（开发环境）
	CorsMethods:       []string{}, // 默认空数组，使用默认方法列表
	CorsHeaders:       []string{}, // 默认空数组，表示允许所有请求头（使用 "*"）
	CorsExposeHeaders: []string{}, // 默认空数组，表示暴露所有响应头（使用 "*"）
	CorsMaxAge:        43200,      // 默认 12 小时（43200 秒）
	CorsCredentials:   false,      // 默认不允许携带凭证
}

func getDefaultPath() (path string) {
	// 始终使用二进制文件所在目录作为 BasePath
	// 如果获取失败，使用 /tmp 作为后备方案（仅用于初始化，实际不会发生）
	path, err := utils.GetDefaultPath()
	if err != nil || path == "" {
		// 如果获取失败，使用临时目录（这种情况不应该发生）
		path = "/tmp"
	}
	return
}
