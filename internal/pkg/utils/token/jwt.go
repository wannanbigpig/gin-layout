package token

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

// AdminUserInfo 是写入 JWT 的管理员基础信息。
type AdminUserInfo struct {
	// 可根据需要自行添加字段
	UserID          uint   `json:"user_id"`
	Username        string `json:"username"`
	FullPhoneNumber string `json:"full_phone_number"`
	Email           string `json:"email"`
	Nickname        string `json:"nickname"`
	PhoneNumber     string `json:"phone_number"`
	CountryCode     string `json:"country_code"`
	IsSuperAdmin    uint8  `json:"is_super_admin"`
}

// Generate 生成JWT Token
func Generate(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	cfg := c.GetConfig()

	// 生成签名字符串
	tokenStr, err := token.SignedString([]byte(cfg.Jwt.SecretKey))
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}

// Refresh 刷新JWT Token
func Refresh(claims jwt.Claims) (string, error) {
	return Generate(claims)
}

// Parse 解析token
func Parse(accessToken string, claims jwt.Claims, options ...jwt.ParserOption) error {
	cfg := c.GetConfig()
	token, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (i interface{}, err error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.Jwt.SecretKey), nil
	}, options...)
	if err != nil {
		return err
	}

	if !token.Valid {
		return e.NewBusinessError(e.NotLogin)
	}

	return nil
}

// GetAccessToken 获取jwt的Token
func GetAccessToken(authorization string) (accessToken string, err error) {
	if authorization == "" {
		return "", errors.New("authorization header is missing")
	}

	// 检查 Authorization 头的格式
	if !strings.HasPrefix(authorization, "Bearer ") {
		return "", errors.New("invalid Authorization header format")
	}

	// 提取 Token 的值
	accessToken = strings.TrimPrefix(authorization, "Bearer ")
	return
}

// AdminCustomClaims 自定义格式内容
type AdminCustomClaims struct {
	AdminUserInfo
	jwt.RegisteredClaims // 内嵌标准的声明
}

// NewAdminCustomClaims 初始化AdminCustomClaims
func NewAdminCustomClaims(user *model.AdminUser) AdminCustomClaims {
	cfg := c.GetConfig()
	now := time.Now().UTC()
	expiresAt := now.Add(time.Second * cfg.Jwt.TTL)
	// phoneRule := &utils.DesensitizeRule{KeepPrefixLen: 3, KeepSuffixLen: 4, MaskChar: '*'}
	// emailRule := &utils.DesensitizeRule{KeepPrefixLen: 2, KeepSuffixLen: 0, MaskChar: '*', Separator: '@', FixedMaskLength: 3}
	return AdminCustomClaims{
		AdminUserInfo: AdminUserInfo{
			UserID:          user.ID,
			Username:        user.Username,
			FullPhoneNumber: user.FullPhoneNumber, // phoneRule.Apply(user.Mobile),
			PhoneNumber:     user.PhoneNumber,     // phoneRule.Apply(user.Mobile),
			CountryCode:     user.CountryCode,     // phoneRule.Apply(user.Mobile),
			Email:           user.Email,           // emailRule.Apply(user.Email),
			Nickname:        user.Nickname,
			IsSuperAdmin:    user.IsSuperAdmin,
		},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt), // 定义过期时间
			Issuer:    global.Issuer,                 // 签发人
			IssuedAt:  jwt.NewNumericDate(now),       // 签发时间
			Subject:   global.PcAdminSubject,         // 签发主题
			NotBefore: jwt.NewNumericDate(now),       // 生效时间
			ID:        uuid.New().String(),           // 唯一标识
		},
	}
}
