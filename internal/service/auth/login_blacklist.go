package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"go.uber.org/zap"
)

// Logout 退出登录。
func (s *LoginService) Logout(accessToken string) error {
	claims, err := s.parseToken(accessToken)
	if err != nil {
		return err
	}

	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return err
	}

	remainingTime := time.Until(exp.Time)
	blacklistKey := s.getBlacklistKey(claims.ID)
	if err := data.RedisClient().Set(context.Background(), blacklistKey, "1", remainingTime).Err(); err != nil {
		return err
	}

	s.revokeTokenInLogAsync([]string{claims.ID}, model.RevokedCodeUserLogout, "用户主动登出（退出登录）")
	return nil
}

// parseToken 解析 Token。
func (s *LoginService) parseToken(accessToken string) (*token.AdminCustomClaims, error) {
	claims := new(token.AdminCustomClaims)
	err := token.Parse(accessToken, claims, jwt.WithSubject(global.PcAdminSubject), jwt.WithIssuer(global.Issuer))
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// IsInBlacklist 判断 Token 是否在黑名单中。
func (s *LoginService) IsInBlacklist(jwtId string) (bool, error) {
	redisClient := data.RedisClient()
	if redisClient == nil {
		if err := data.GetRedisInitError(); err != nil {
			return false, err
		}
		return false, errors.New("redis client is not available")
	}
	result, err := redisClient.Exists(context.Background(), s.getBlacklistKey(jwtId)).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// getBlacklistKey 获取 Redis 黑名单 key。
func (s *LoginService) getBlacklistKey(jwtId string) string {
	return blacklistPrefix + jwtId
}

// addTokensToBlacklist 批量将 token 加入 Redis 黑名单。
func (s *LoginService) addTokensToBlacklist(loginLogs []model.AdminLoginLogs) {
	if len(loginLogs) == 0 {
		return
	}

	ctx := context.Background()
	redisClient := data.RedisClient()
	if redisClient == nil {
		return
	}

	now := time.Now()
	for _, item := range loginLogs {
		if item.JwtID == "" {
			continue
		}
		remainingTime := s.calculateRemainingTime(item.TokenExpires, now)
		if remainingTime <= 0 {
			continue
		}
		blacklistKey := s.getBlacklistKey(item.JwtID)
		if err := redisClient.Set(ctx, blacklistKey, "1", remainingTime).Err(); err != nil {
			log.Logger.Error("将token加入Redis黑名单失败", zap.Error(err), zap.String("jwt_id", item.JwtID))
		}
	}
}

// calculateRemainingTime 计算 token 剩余过期时间。
func (s *LoginService) calculateRemainingTime(tokenExpires *utils.FormatDate, now time.Time) time.Duration {
	if tokenExpires != nil {
		remainingTime := tokenExpires.Time.Sub(now)
		if remainingTime > 0 {
			return remainingTime
		}
		return 0
	}
	return defaultTokenTTL
}

// RevokeUserTokens 撤销用户所有未过期的 token。
func (s *LoginService) RevokeUserTokens(userId uint, revokedCode uint8, revokedReason string, tx ...*gorm.DB) error {
	if userId == 0 {
		return nil
	}

	loginLog := model.NewAdminLoginLogs()
	db, err := s.loginLogDB(loginLog, tx...)
	if err != nil {
		return err
	}

	loginLogs, err := s.activeLoginLogs(userId, db)
	if err != nil || len(loginLogs) == 0 {
		return err
	}

	jwtIds := collectJWTIDs(loginLogs)
	if len(jwtIds) == 0 {
		return nil
	}

	s.addTokensToBlacklist(loginLogs)
	s.revokeTokenInLogAsync(jwtIds, revokedCode, revokedReason)
	return nil
}

func (s *LoginService) loginLogDB(loginLog *model.AdminLoginLogs, tx ...*gorm.DB) (*gorm.DB, error) {
	if len(tx) > 0 && tx[0] != nil {
		loginLog.SetDB(tx[0])
		return tx[0], nil
	}
	return loginLog.GetDB()
}

func (s *LoginService) activeLoginLogs(userId uint, db *gorm.DB) ([]model.AdminLoginLogs, error) {
	var loginLogs []model.AdminLoginLogs
	now := time.Now()
	err := db.Where("uid = ? AND deleted_at = 0 AND is_revoked = ? AND login_status = ? AND token_expires IS NOT NULL AND token_expires > ?",
		userId, model.IsRevokedNo, model.LoginStatusSuccess, now).Find(&loginLogs).Error
	if err != nil {
		log.Logger.Error("查询用户未过期token失败", zap.Error(err), zap.Uint("user_id", userId))
	}
	return loginLogs, err
}

func collectJWTIDs(loginLogs []model.AdminLoginLogs) []string {
	jwtIds := make([]string, 0, len(loginLogs))
	for _, item := range loginLogs {
		if item.JwtID != "" {
			jwtIds = append(jwtIds, item.JwtID)
		}
	}
	return jwtIds
}
