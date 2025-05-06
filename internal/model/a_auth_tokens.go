package model

import "github.com/wannanbigpig/gin-layout/internal/pkg/utils"

// AuthTokens 用户认证令牌及登录日志表
type AuthTokens struct {
	ContainsDeleteBaseModel
	UID              uint              `json:"uid"`                // 用户ID
	UserType         uint8             `json:"user_type"`          // 用户类型：1=管理员(admin_users表), 2=普通用户(users表)
	ClientType       uint8             `json:"client_type"`        // 客户端类型：1=Web, 2=iOS, 3=Android, 4=小程序
	DeviceID         string            `json:"device_id"`          // 设备唯一标识
	DeviceName       string            `json:"device_name"`        // 设备名称(如iPhone 15)
	JwtID            string            `json:"jwt_id"`             // JWT唯一标识(jti claim)
	AccessToken      string            `json:"access_token"`       // 访问令牌
	RefreshToken     string            `json:"refresh_token"`      // 刷新令牌
	TokenHash        string            `json:"token_hash"`         // Token的SHA256哈希值
	RefreshTokenHash string            `json:"refresh_token_hash"` // Refresh Token的哈希值
	IP               string            `json:"ip"`                 // 登录IP(支持IPv6)
	IsRevoked        uint8             `json:"is_revoked"`         // 是否被撤销：0=否, 1=是
	RevokedCode      uint8             `json:"revoked_code"`       // 撤销原因码：1=用户主动登出（退出登录）, 2=系统强制登出（账号被封）, 3=系统刷新token, 4=用户禁用（针对某个设备下线操作） 5=其他原因
	RevokedReason    string            `json:"revoked_reason"`     // 撤销原因
	RevokedAt        *utils.FormatDate `json:"revoked_at"`         // 撤销时间
	TokenExpires     *utils.FormatDate `json:"token_expires"`      // Token过期时间
	RefreshExpires   *utils.FormatDate `json:"refresh_expires"`    // Refresh Token过期时间
}

func NewAuthTokens() *AuthTokens {
	return &AuthTokens{}
}

// TableName 获取表名
func (m *AuthTokens) TableName() string {
	return "a_auth_tokens"
}
