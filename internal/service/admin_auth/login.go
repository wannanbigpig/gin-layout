package admin_auth

import (
	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"time"
)

// TokenResponse token响应体
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresAt   int64  `json:"expires_at"`
}

// LoginService 登录授权服务
type LoginService struct {
	service.Base
}

func NewLoginService() *LoginService {
	return &LoginService{}
}

func (s *LoginService) Login(username, password string) (*TokenResponse, error) {
	adminUserModel := model.NewAdminUsers()
	// 查询用户是否存在
	user := adminUserModel.GetUserInfo(username)

	if user == nil {
		err := e.NewBusinessError(e.UserDoesNotExist)
		return nil, err
	}

	// 判断用户状态是否禁用
	if user.Status != 1 {
		err := e.NewBusinessError(e.UserDoesNotExist)
		return nil, err
	}

	// 校验密码
	if !adminUserModel.ComparePasswords(password) {
		return nil, e.NewBusinessError(e.FAILURE, "用户密码错误")
	}
	claims := s.newAdminCustomClaims(user)
	accessToken, err := token.Generate(claims)
	if err != nil {
		return nil, e.NewBusinessError(e.FAILURE, "生成Token失败")
	}

	return &TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresAt:   claims.ExpiresAt.Unix(),
	}, nil
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
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresAt:   claims.ExpiresAt.Unix(),
	}, nil
}

// newAdminCustomClaims 初始化AdminCustomClaims
func (s *LoginService) newAdminCustomClaims(user *model.AdminUser) token.AdminCustomClaims {
	now := time.Now()
	expiresAt := now.Add(time.Second * c.Config.Jwt.TTL)
	return token.NewAdminCustomClaims(user, expiresAt)
}
