package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
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

// isTokenRevokedInLog 检查登录日志表中 token 是否被撤销。
func (s *LoginService) isTokenRevokedInLog(jwtId string) bool {
	if jwtId == "" {
		return false
	}

	loginLog := model.NewAdminLoginLogs()
	db, err := loginLog.GetDB()
	if err != nil {
		return false
	}
	err = db.Where("jwt_id = ? AND deleted_at = 0", jwtId).Select("is_revoked").First(loginLog).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Logger.Error("检查token撤销状态失败", zap.Error(err), zap.String("jwt_id", jwtId))
		}
		return false
	}

	return loginLog.IsRevoked == model.IsRevokedYes
}

// revokeTokenInLog 在登录日志表中标记 token 被撤销。
// RevokeUserTokens 撤销用户所有未过期的 token。
func (s *LoginService) RevokeUserTokens(userId uint, revokedCode uint8, revokedReason string, tx ...*gorm.DB) error {
	if userId == 0 {
		return nil
	}

	loginLog := model.NewAdminLoginLogs()
	now := time.Now()
	var db *gorm.DB
	var err error
	if len(tx) > 0 && tx[0] != nil {
		db = tx[0]
		loginLog.SetDB(tx[0])
	} else {
		db, err = loginLog.GetDB()
		if err != nil {
			return err
		}
	}

	var loginLogs []model.AdminLoginLogs
	err = db.Where("uid = ? AND deleted_at = 0 AND is_revoked = ? AND login_status = ? AND token_expires IS NOT NULL AND token_expires > ?",
		userId, model.IsRevokedNo, model.LoginStatusSuccess, now).Find(&loginLogs).Error
	if err != nil {
		log.Logger.Error("查询用户未过期token失败", zap.Error(err), zap.Uint("user_id", userId))
		return err
	}
	if len(loginLogs) == 0 {
		return nil
	}

	jwtIds := make([]string, 0, len(loginLogs))
	for _, item := range loginLogs {
		if item.JwtID != "" {
			jwtIds = append(jwtIds, item.JwtID)
		}
	}
	if len(jwtIds) == 0 {
		return nil
	}

	s.addTokensToBlacklist(loginLogs)
	s.revokeTokenInLogAsync(jwtIds, revokedCode, revokedReason)
	return nil
}

func (s *LoginService) revokeTokenInLogAsync(jwtIds []string, revokedCode uint8, revokedReason string) {
	if len(jwtIds) == 0 {
		return
	}

	go func(ids []string) {
		if err := s.markTokensRevoked(ids, revokedCode, revokedReason); err != nil {
			log.Logger.Error("异步更新登录日志 token 撤销状态失败", zap.Error(err), zap.Strings("jwt_ids", ids))
		}
	}(append([]string(nil), jwtIds...))
}

func (s *LoginService) markTokensRevoked(jwtIds []string, revokedCode uint8, revokedReason string) error {
	now := time.Now()
	revokedAt := utils.FormatDate{Time: now}
	updates := map[string]interface{}{
		"is_revoked":     model.IsRevokedYes,
		"revoked_code":   revokedCode,
		"revoked_reason": revokedReason,
		"revoked_at":     revokedAt,
		"updated_at":     now,
	}

	loginLog := model.NewAdminLoginLogs()
	db, err := loginLog.GetDB(loginLog)
	if err != nil {
		return err
	}

	return db.
		Where("jwt_id IN ? AND deleted_at = 0 AND is_revoked = ?", jwtIds, model.IsRevokedNo).
		Updates(updates).Error
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

// CheckToken 检查 Token 是否有效。
func (s *LoginService) CheckToken(accessToken string) (*model.AdminUser, bool) {
	claims, err := s.parseToken(accessToken)
	if err != nil {
		return nil, false
	}
	if !s.isTokenValid(claims) {
		return nil, false
	}

	s.tryRefreshToken(claims)

	adminUser, err := s.getUserById(claims.UserID)
	if err != nil {
		return nil, false
	}
	if !s.isUserValid(adminUser, claims.ID) {
		return adminUser, false
	}
	return adminUser, true
}

// isTokenValid 检查 Token 是否未过期。
func (s *LoginService) isTokenValid(claims *token.AdminCustomClaims) bool {
	exp, err := claims.GetExpirationTime()
	return err == nil && exp != nil
}

// isUserValid 检查用户是否有效。
func (s *LoginService) isUserValid(adminUser *model.AdminUser, jwtId string) bool {
	if adminUser.Status != model.AdminUserStatusEnabled {
		return false
	}
	inBlacklist, err := s.IsInBlacklist(jwtId)
	if err == nil {
		return !inBlacklist
	}

	log.Logger.Warn("Redis 黑名单查询失败，回退到数据库撤销状态校验", zap.Error(err), zap.String("jwt_id", jwtId))
	return !s.isTokenRevokedInLog(jwtId)
}
