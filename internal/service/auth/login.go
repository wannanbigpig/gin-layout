package auth

import (
	stderrors "errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

// LoginService 登录授权服务。
type LoginService struct {
	service.Base
}

// NewLoginService 创建登录服务实例。
func NewLoginService() *LoginService {
	return &LoginService{}
}

// Login 用户登录。
func (s *LoginService) Login(username, password string, logInfo LoginLogInfo) (*TokenResponse, error) {
	startTime := time.Now()

	adminUser, err := s.validateUser(username, password)
	if err != nil {
		logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
		s.RecordLoginFailLog(username, s.extractErrorMessage(err), logInfo)
		return nil, err
	}

	claims := s.newAdminCustomClaims(adminUser)
	accessToken, err := token.Generate(claims)
	if err != nil {
		logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
		s.RecordLoginFailLog(username, "生成Token失败", logInfo)
		return nil, e.NewBusinessError(e.TokenGenerateFailed)
	}

	logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
	if err := s.recordLoginLog(adminUser, claims, accessToken, "", logInfo, model.LoginTypeLogin); err != nil {
		return nil, e.NewBusinessError(e.LoginFailed)
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: "",
		TokenType:    tokenTypeBearer,
		ExpiresAt:    claims.ExpiresAt.Unix(),
	}, nil
}

// validateUser 验证用户信息。
func (s *LoginService) validateUser(username, password string) (*model.AdminUser, error) {
	adminUser := model.NewAdminUsers()
	if err := adminUser.GetUserInfo(username); err != nil {
		switch {
		case e.IsDependencyNotReady(err):
			return nil, e.NewDependencyNotReadyError()
		case stderrors.Is(err, gorm.ErrRecordNotFound):
			return nil, e.NewBusinessError(e.UserDoesNotExist)
		default:
			return nil, err
		}
	}
	if adminUser.Status != model.AdminUserStatusEnabled {
		return nil, e.NewBusinessError(e.UserDisable)
	}
	if !utils2.ComparePasswords(adminUser.Password, password) {
		return nil, e.NewBusinessError(e.UserPasswordWrong)
	}
	return adminUser, nil
}

// recordLoginLog 记录登录日志并更新用户信息。
func (s *LoginService) recordLoginLog(adminUser *model.AdminUser, claims token.AdminCustomClaims, accessToken, refreshToken string, logInfo LoginLogInfo, logType uint8) error {
	db, err := model.NewAdminLoginLogs().GetDB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		loginLog := s.buildLoginLog(adminUser.ID, adminUser.Username, claims.ID, accessToken, refreshToken, claims.ExpiresAt.Time, logInfo, model.LoginStatusSuccess, "", logType)
		loginLog.SetDB(tx)
		if err := loginLog.Create(); err != nil {
			log.Logger.Error("记录登录日志失败", zap.Error(err), zap.Uint("user_id", adminUser.ID), zap.String("username", adminUser.Username))
			return err
		}

		if logType == model.LoginTypeLogin {
			adminUser.LastIp = logInfo.IP
			adminUser.LastLogin = utils.FormatDate{Time: time.Now()}
			adminUser.SetDB(tx)
			if err := adminUser.Save(); err != nil {
				log.Logger.Error("更新用户最后登录信息失败", zap.Error(err), zap.Uint("user_id", adminUser.ID))
				return err
			}
		}
		return nil
	})
}

// Refresh 刷新 Token。
func (s *LoginService) Refresh(id uint, logInfo LoginLogInfo) (*TokenResponse, error) {
	startTime := time.Now()

	adminUserModel := model.NewAdminUsers()
	if err := adminUserModel.GetById(id); err != nil {
		return nil, e.NewBusinessError(e.UpdateUserFailed)
	}

	claims := s.newAdminCustomClaims(adminUserModel)
	accessToken, err := token.Refresh(claims)
	if err != nil {
		return nil, e.NewBusinessError(e.TokenGenerateFailed)
	}

	logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
	if err := s.recordLoginLog(adminUserModel, claims, accessToken, "", logInfo, model.LoginTypeRefresh); err != nil {
		log.Logger.Error("记录刷新token日志失败", zap.Error(err), zap.Uint("user_id", id))
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: "",
		TokenType:    tokenTypeBearer,
		ExpiresAt:    claims.ExpiresAt.Unix(),
	}, nil
}

// newAdminCustomClaims 创建管理员自定义 Claims。
func (s *LoginService) newAdminCustomClaims(user *model.AdminUser) token.AdminCustomClaims {
	return token.NewAdminCustomClaims(user)
}
