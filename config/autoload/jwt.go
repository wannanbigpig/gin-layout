package autoload

import "time"

type JwtConfig struct {
	TTL time.Duration `mapstructure:"ttl"`
	// 默认0，不主动刷新 Token
	// 刷新时间大于0，则判断剩余过期时间小于刷新时间时刷新 Token 并在 Response Header 中返回
	// 如果刷新时间大于过期时间则实时刷新，推荐刷新时间设置为 TTL/2
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
	SecretKey  string        `mapstructure:"secret_key"`
}

var Jwt = JwtConfig{
	TTL:        7200,
	RefreshTTL: 0,
	SecretKey:  "",
}
