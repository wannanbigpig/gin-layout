package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/pkg/utils/crypto"
)

// buildLoginLog 构建登录日志记录。
func (s *LoginService) buildLoginLog(uid uint, username, jwtId, accessToken, refreshToken string, expiresAt time.Time, logInfo LoginLogInfo, loginStatus uint8, failReason string, logType uint8) *model.AdminLoginLogs {
	loginLog := model.NewAdminLoginLogs()
	loginLog.UID = uid
	loginLog.Username = username
	loginLog.JwtID = jwtId
	s.encryptAndSetToken(loginLog, accessToken, refreshToken, uid)
	loginLog.IP = logInfo.IP
	loginLog.UserAgent = logInfo.UserAgent
	loginLog.OS = logInfo.OS
	loginLog.Browser = logInfo.Browser
	loginLog.ExecutionTime = logInfo.ExecutionTime
	loginLog.LoginStatus = loginStatus
	loginLog.LoginFailReason = failReason
	loginLog.Type = logType
	if !expiresAt.IsZero() {
		loginLog.TokenExpires = &(utils.FormatDate{Time: expiresAt})
	}
	return loginLog
}

// encryptAndSetToken 加密并设置 token 到登录日志。
func (s *LoginService) encryptAndSetToken(loginLog *model.AdminLoginLogs, accessToken, refreshToken string, uid uint) {
	encryptKey := config.GetConfig().Jwt.SecretKey
	if accessToken != "" {
		loginLog.AccessToken = s.encryptToken(encryptKey, accessToken, "access_token", uid)
		loginLog.TokenHash = s.calculateTokenHash(accessToken)
	}
	if refreshToken != "" {
		loginLog.RefreshToken = s.encryptToken(encryptKey, refreshToken, "refresh_token", uid)
		loginLog.RefreshTokenHash = s.calculateTokenHash(refreshToken)
	}
}

// encryptToken 加密 token。
func (s *LoginService) encryptToken(key, token, tokenType string, uid uint) string {
	encrypted, err := crypto.Encrypt(key, token)
	if err != nil {
		log.Logger.Error("加密 token 失败", zap.Error(err), zap.String("token_type", tokenType), zap.Uint("user_id", uid))
		return ""
	}
	return encrypted
}

// extractErrorMessage 提取简洁的错误消息。
func (s *LoginService) extractErrorMessage(err error) string {
	var businessErr *e.BusinessError
	if errors.As(err, &businessErr) {
		return businessErr.GetMessage()
	}
	return err.Error()
}

// RecordLoginFailLog 记录登录失败日志。
func (s *LoginService) RecordLoginFailLog(username, failReason string, logInfo LoginLogInfo) {
	if !config.GetConfig().Mysql.Enable {
		return
	}

	loginLog := s.buildLoginLog(0, username, "", "", "", time.Time{}, logInfo, model.LoginStatusFail, failReason, model.LoginTypeLogin)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logLoginAsyncError("记录登录失败日志 panic",
					zap.String("operation", "record_login_fail_log"),
					zap.String("username", username),
					zap.String("fail_reason", failReason),
					zap.Any("recover", r))
			}
		}()
		if err := loginLog.Create(); err != nil {
			logLoginAsyncError("记录登录失败日志出错",
				zap.String("operation", "record_login_fail_log"),
				zap.String("username", username),
				zap.String("fail_reason", failReason),
				zap.String("ip", logInfo.IP),
				zap.Error(err))
		}
	}()
}

// calculateTokenHash 计算 Token 的 SHA256 哈希值。
func (s *LoginService) calculateTokenHash(accessToken string) string {
	hashBytes := sha256.Sum256([]byte(accessToken))
	return hex.EncodeToString(hashBytes[:])
}

func logLoginAsyncError(message string, fields ...zap.Field) {
	if log.Logger == nil {
		return
	}
	log.Logger.Error(message, fields...)
}
