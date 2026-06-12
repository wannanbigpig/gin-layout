package autoload

import "time"

// JwtConfig 定义 JWT 相关配置。
type JwtConfig struct {
	// TTL Access Token 有效期（秒），默认 1800 秒（30 分钟）。
	TTL time.Duration `mapstructure:"ttl"`
	// RefreshTTL 旧版自动刷新阈值（已废弃，新双 Token 方案不再使用）。
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
	// RefreshTokenTTL Refresh Token 有效期（秒），默认 7 天。
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
	// RefreshCookieName Refresh Token Cookie 名称。
	RefreshCookieName string `mapstructure:"refresh_cookie_name"`
	// RefreshCookiePath Refresh Token Cookie 路径，按浏览器可见 API 路径配置。
	RefreshCookiePath string `mapstructure:"refresh_cookie_path"`
	// RefreshCookieDomain Refresh Token Cookie 域，空表示当前 host。
	RefreshCookieDomain string `mapstructure:"refresh_cookie_domain"`
	// RefreshCookieSecure 是否仅通过 HTTPS 发送 Refresh Token Cookie。
	RefreshCookieSecure bool `mapstructure:"refresh_cookie_secure"`
	// RefreshCookieSameSite Cookie SameSite 策略：Lax、Strict、None。
	RefreshCookieSameSite string `mapstructure:"refresh_cookie_same_site"`
	// SecretKey JWT 签名密钥，用于生成和验证 Token
	// 启动时会校验非空；生产环境还会拒绝弱占位值和长度不足的密钥
	// 建议使用随机密钥，例如：openssl rand -hex 32
	SecretKey string `mapstructure:"secret_key"`
}

var Jwt = JwtConfig{
	TTL:                   1800,
	RefreshTTL:            0,
	RefreshTokenTTL:       604800,
	RefreshCookieName:     "refresh_token",
	RefreshCookiePath:     "/admin/v1/auth",
	RefreshCookieDomain:   "",
	RefreshCookieSecure:   false,
	RefreshCookieSameSite: "Lax",
	SecretKey:             "",
}
