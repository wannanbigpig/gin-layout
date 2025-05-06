package permission

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

// TokenResponse token响应体
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresAt    int64  `json:"expires_at"`
}

// LoginService 登录授权服务
type LoginService struct {
	service.Base
}

func NewLoginService() *LoginService {
	return &LoginService{}
}

type LoginLogInfo struct {
	IP         string `json:"ip"`
	DeviceId   string `json:"device_id"`
	ClientType uint8  `json:"ClientType"`
	DeviceName string `json:"device_name"`
}

func (s *LoginService) Login(username, password string, logInfo LoginLogInfo) (*TokenResponse, error) {
	adminUserModel := model.NewAdminUsers()
	// 查询用户是否存在
	user := adminUserModel.GetUserInfo(username)

	if user == nil {
		err := e.NewBusinessError(e.UserDoesNotExist)
		return nil, err
	}

	// 判断用户状态是否禁用
	if user.Status != 1 {
		err := e.NewBusinessError(e.UserDisable)
		return nil, err
	}

	// 校验密码
	if !utils2.ComparePasswords(user.Password, password) {
		return nil, e.NewBusinessError(e.FAILURE, "用户密码错误")
	}
	claims := s.newAdminCustomClaims(user)
	accessToken, err := token.Generate(claims)
	if err != nil {
		return nil, e.NewBusinessError(e.FAILURE, "生成Token失败")
	}

	err = model.DB().Transaction(func(tx *gorm.DB) error {
		err = s.AuthTokenLog(1, user.ID, claims.ID, accessToken, claims.ExpiresAt.Time, logInfo, tx)
		if err != nil {
			return err
		}

		user.LastIp = logInfo.IP
		user.LastLogin = utils.FormatDate{Time: time.Now()}
		tx.Save(user)
		return nil
	})

	if err != nil {
		return nil, e.NewBusinessError(e.FAILURE, "记录授权日志出错，登录失败")
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: "",
		TokenType:    "Bearer",
		ExpiresAt:    claims.ExpiresAt.Unix(),
	}, nil
}

// AuthTokenLog 授权日志
func (s *LoginService) AuthTokenLog(userType uint8, uid uint, jwtId, accessToken string, expiresAt time.Time, logInfo LoginLogInfo, db ...*gorm.DB) error {
	authTokensModel := model.NewAuthTokens()
	authTokensModel.UID = uid
	authTokensModel.UserType = userType
	authTokensModel.JwtID = jwtId
	authTokensModel.AccessToken = accessToken
	// 计算SHA256哈希值（返回32字节数组）
	hashBytes := sha256.Sum256([]byte(accessToken))
	authTokensModel.TokenHash = hex.EncodeToString(hashBytes[:])
	authTokensModel.IP = logInfo.IP
	authTokensModel.DeviceID = logInfo.DeviceId
	authTokensModel.DeviceName = logInfo.DeviceName
	authTokensModel.ClientType = logInfo.ClientType
	authTokensModel.TokenExpires = &(utils.FormatDate{Time: expiresAt})
	tx := authTokensModel.DB()
	if len(db) > 0 {
		tx = db[0]
	}
	err := tx.Create(authTokensModel).Error
	return err
}

// Refresh 刷新Token
func (s *LoginService) Refresh(id uint) (*TokenResponse, error) {
	// 查询用户是否存在
	adminUserModel := model.NewAdminUsers()
	user := adminUserModel.GetUserById(id)
	if user == nil {
		return nil, e.NewBusinessError(e.FAILURE, "更新用户异常")
	}

	claims := s.newAdminCustomClaims(user)
	accessToken, err := token.Refresh(claims)
	if err != nil {
		return nil, e.NewBusinessError(e.FAILURE, "生成Token失败")
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: "",
		TokenType:    "Bearer",
		ExpiresAt:    claims.ExpiresAt.Unix(),
	}, nil
}

// newAdminCustomClaims 初始化AdminCustomClaims
func (s *LoginService) newAdminCustomClaims(user *model.AdminUser) token.AdminCustomClaims {
	return token.NewAdminCustomClaims(user)
}

// Logout 退出登录
func (s *LoginService) Logout(accessToken string) error {
	// Get the expiration time of the token
	adminCustomClaims := new(token.AdminCustomClaims)
	err := token.Parse(accessToken, adminCustomClaims, jwt.WithSubject(global.PcAdminSubject), jwt.WithIssuer(global.Issuer))
	if err != nil {
		return err
	}

	exp, err := adminCustomClaims.GetExpirationTime()
	if err != nil || exp == nil {
		return err
	}

	// Calculate the remaining time of the token
	remainingTime := exp.Time.Sub(time.Now())
	fmt.Println("退出", accessToken)
	// Add the token to the Redis blacklist
	err = data.Rdb.Set(context.Background(), s.getBlacklistKey(adminCustomClaims.ID), "1", remainingTime).Err()
	if err != nil {
		return err
	}

	return nil
}

// IsInBlacklist 判断Token是否在黑名单中
func (s *LoginService) IsInBlacklist(jwtId string) bool {
	// 检查token是否在Redis黑名单中
	result, err := data.Rdb.Exists(context.Background(), s.getBlacklistKey(jwtId)).Result()
	if err != nil {
		return false
	}
	return result > 0
}

// getBlacklistKey 统一获取 Redis 黑名单的 key
func (s *LoginService) getBlacklistKey(jwtId string) string {
	// 定义常量，避免硬编码
	const prefix = "blacklist:"

	// 使用 fmt.Sprintf 简化字符串拼接，同时保持性能和可读性
	return prefix + jwtId
}
