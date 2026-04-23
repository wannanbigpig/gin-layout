package autoload

import "time"

// JwtConfig 定义 JWT 相关配置。
type JwtConfig struct {
	// TTL Token 有效期（秒），默认 7200 秒（2 小时）
	TTL time.Duration `mapstructure:"ttl"`
	// RefreshTTL Token 刷新阈值（秒）。
	// 默认 0：不主动刷新 Token
	// 大于 0 时：当 Token 剩余有效期小于该值时，自动刷新 Token 并在 Response Header 中返回新 Token
	// 推荐设置为 TTL/2，例如 TTL=7200 时，RefreshTTL=3600
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
	// SecretKey JWT 签名密钥，用于生成和验证 Token
	// 启动时会校验非空；生产环境还会拒绝弱占位值和长度不足的密钥
	// 建议使用随机密钥，例如：openssl rand -hex 32
	SecretKey string `mapstructure:"secret_key"`
}

var Jwt = JwtConfig{
	TTL:        7200, // Token 有效期 2 小时
	RefreshTTL: 0,    // 0 表示不主动刷新 Token
	SecretKey:  "",   // 默认空，启动时必须由配置提供有效密钥
}
