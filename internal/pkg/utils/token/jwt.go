package token

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"strings"
	"time"
)

type AdminUserInfo struct {
	// 可根据需要自行添加字段
	UserID   uint   `json:"user_id"`
	Mobile   string `json:"mobile"`
	Nickname string `json:"nickname"`
}

// GetAdminUserInfo 把传入数据转换成AdminUserInfo结构体
func GetAdminUserInfo(info any) (adminUserInfo AdminUserInfo) {
	adminUserInfo, _ = info.(AdminUserInfo)
	return
}

// Generate 生成JWT Token
func Generate(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 生成签名字符串
	tokenStr, err := token.SignedString([]byte(c.Config.Jwt.SecretKey))
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
	token, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (i interface{}, err error) {
		return []byte(c.Config.Jwt.SecretKey), err
	}, options...)
	if err != nil {
		return err
	}

	// 对token对象中的Claim进行类型断言
	if token.Valid { // 校验token
		return nil
	}

	return e.NewBusinessError(1, "invalid token")
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
func NewAdminCustomClaims(user *model.AdminUsers, expiresAt time.Time) AdminCustomClaims {
	//now := time.Now()
	return AdminCustomClaims{
		AdminUserInfo: AdminUserInfo{
			user.ID,
			user.Mobile,
			user.Nickname,
		},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt), // 定义过期时间
			Issuer:    global.Issuer,                 // 签发人
			//IssuedAt:  jwt.NewNumericDate(now),       // 签发时间
			Subject: global.Subject, // 签发主体
			//NotBefore: jwt.NewNumericDate(now),       // 生效时间
		},
	}
}
