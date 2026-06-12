package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	tokenTypeBearer = "Bearer"
	blacklistPrefix = "blacklist:"
	defaultTokenTTL = 24 * time.Hour // 默认 token 过期时间（当 token_expires 为 NULL 时使用）
)

// TokenResponse Token响应体。
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"-"`
	TokenType    string `json:"token_type"`
	ExpiresAt    int64  `json:"expires_at"`
}

type RefreshClaims struct {
	UserID   uint   `json:"user_id"`
	TokenID  string `json:"token_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// LoginLogInfo 登录日志信息。
type LoginLogInfo struct {
	IP            string `json:"ip"`
	UserAgent     string `json:"user_agent"`
	OS            string `json:"os"`
	Browser       string `json:"browser"`
	ExecutionTime int    `json:"execution_time"`
}
