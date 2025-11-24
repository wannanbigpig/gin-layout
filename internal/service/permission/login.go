package permission

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mssola/useragent"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

const (
	tokenTypeBearer = "Bearer"
	blacklistPrefix = "blacklist:"
)

// TokenResponse Token响应体
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresAt    int64  `json:"expires_at"`
}

// LoginLogInfo 登录日志信息
type LoginLogInfo struct {
	IP            string `json:"ip"`
	UserAgent     string `json:"user_agent"`     // 用户代理（浏览器/设备信息）
	OS            string `json:"os"`             // 操作系统
	Browser       string `json:"browser"`        // 浏览器
	ExecutionTime int    `json:"execution_time"` // 登录耗时（毫秒）
}

// LoginService 登录授权服务
type LoginService struct {
	service.Base
}

// NewLoginService 创建登录服务实例
func NewLoginService() *LoginService {
	return &LoginService{}
}

// Login 用户登录
func (s *LoginService) Login(username, password string, logInfo LoginLogInfo) (*TokenResponse, error) {
	startTime := time.Now()

	// 验证用户信息
	adminUser, err := s.validateUser(username, password)
	if err != nil {
		// 记录登录失败日志
		logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
		// 提取简洁的错误消息
		failReason := s.extractErrorMessage(err)
		s.RecordLoginFailLog(username, failReason, logInfo)
		return nil, err
	}

	// 生成Token
	claims := s.newAdminCustomClaims(adminUser)
	accessToken, err := token.Generate(claims)
	if err != nil {
		// 记录登录失败日志（Token生成失败）
		logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
		s.RecordLoginFailLog(username, "生成Token失败", logInfo)
		return nil, e.NewBusinessError(e.FAILURE, "生成Token失败")
	}

	// 计算执行时间
	logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())

	// 记录登录日志并更新用户信息
	if err := s.recordLoginLog(adminUser, claims, accessToken, logInfo, model.LoginTypeLogin); err != nil {
		return nil, e.NewBusinessError(e.FAILURE, "登录失败，请稍后重试")
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: "",
		TokenType:    tokenTypeBearer,
		ExpiresAt:    claims.ExpiresAt.Unix(),
	}, nil
}

// validateUser 验证用户信息
func (s *LoginService) validateUser(username, password string) (*model.AdminUser, error) {
	adminUser := model.NewAdminUsers()
	if err := adminUser.GetUserInfo(username); err != nil {
		return nil, e.NewBusinessError(e.UserDoesNotExist)
	}

	// 检查用户状态
	if adminUser.Status != int8(model.AdminUserStatusEnabled) {
		return nil, e.NewBusinessError(e.UserDisable)
	}

	// 验证密码
	if !utils2.ComparePasswords(adminUser.Password, password) {
		return nil, e.NewBusinessError(e.FAILURE, "用户密码错误")
	}

	return adminUser, nil
}

// recordLoginLog 记录登录日志并更新用户信息
func (s *LoginService) recordLoginLog(adminUser *model.AdminUser, claims token.AdminCustomClaims, accessToken string, logInfo LoginLogInfo, logType uint8) error {
	return model.DB().Transaction(func(tx *gorm.DB) error {
		// 记录登录成功日志
		loginLog := s.buildLoginLog(adminUser.ID, adminUser.Username, claims.ID, accessToken, claims.ExpiresAt.Time, logInfo, model.LoginStatusSuccess, "", logType)
		loginLog.SetDB(tx)
		if err := tx.Create(loginLog).Error; err != nil {
			log.Logger.Error("记录登录日志失败", zap.Error(err), zap.Uint("user_id", adminUser.ID), zap.String("username", adminUser.Username))
			return err
		}

		// 更新用户最后登录信息（仅登录操作时更新）
		if logType == model.LoginTypeLogin {
			adminUser.LastIp = logInfo.IP
			adminUser.LastLogin = utils.FormatDate{Time: time.Now()}
			if err := tx.Save(adminUser).Error; err != nil {
				log.Logger.Error("更新用户最后登录信息失败", zap.Error(err), zap.Uint("user_id", adminUser.ID))
				return err
			}
		}
		return nil
	})
}

// buildLoginLog 构建登录日志记录（公共方法，提取重复代码）
func (s *LoginService) buildLoginLog(uid uint, username, jwtId, accessToken string, expiresAt time.Time, logInfo LoginLogInfo, loginStatus uint8, failReason string, logType uint8) *model.AdminLoginLogs {
	loginLog := model.NewAdminLoginLogs()
	loginLog.UID = uid
	loginLog.Username = username
	loginLog.JwtID = jwtId
	loginLog.AccessToken = accessToken
	if accessToken != "" {
		loginLog.TokenHash = s.calculateTokenHash(accessToken)
	}
	loginLog.IP = logInfo.IP
	loginLog.UserAgent = logInfo.UserAgent
	loginLog.OS = logInfo.OS
	loginLog.Browser = logInfo.Browser
	loginLog.ExecutionTime = logInfo.ExecutionTime
	loginLog.LoginStatus = loginStatus
	loginLog.LoginFailReason = failReason
	loginLog.Type = logType
	if expiresAt.IsZero() {
		loginLog.TokenExpires = nil
	} else {
		loginLog.TokenExpires = &(utils.FormatDate{Time: expiresAt})
	}
	return loginLog
}

// extractErrorMessage 提取简洁的错误消息
// 如果是BusinessError，返回GetMessage()，否则返回Error()
func (s *LoginService) extractErrorMessage(err error) string {
	var businessErr *e.BusinessError
	if errors.As(err, &businessErr) {
		return businessErr.GetMessage()
	}
	return err.Error()
}

// RecordLoginFailLog 记录登录失败日志（公开方法，供Controller调用）
func (s *LoginService) RecordLoginFailLog(username, failReason string, logInfo LoginLogInfo) {
	loginLog := s.buildLoginLog(0, username, "", "", time.Time{}, logInfo, model.LoginStatusFail, failReason, model.LoginTypeLogin)

	// 异步记录，避免影响登录响应速度
	go func() {
		// 使用 model.DB() 确保使用全局数据库连接，而不是实例的 dbInstance（可能为 nil）
		db := model.DB()
		if db == nil {
			log.Logger.Error("数据库连接未初始化，无法记录登录失败日志")
			return
		}
		if err := db.Create(loginLog).Error; err != nil {
			log.Logger.Error("记录登录失败日志出错", zap.Error(err))
		}
	}()
}

// calculateTokenHash 计算Token的SHA256哈希值
func (s *LoginService) calculateTokenHash(accessToken string) string {
	hashBytes := sha256.Sum256([]byte(accessToken))
	return hex.EncodeToString(hashBytes[:])
}

// Refresh 刷新Token
func (s *LoginService) Refresh(id uint, logInfo LoginLogInfo) (*TokenResponse, error) {
	startTime := time.Now()

	adminUserModel := model.NewAdminUsers()
	if err := adminUserModel.GetById(adminUserModel, id); err != nil {
		return nil, e.NewBusinessError(e.FAILURE, "更新用户异常")
	}

	claims := s.newAdminCustomClaims(adminUserModel)
	accessToken, err := token.Refresh(claims)
	if err != nil {
		return nil, e.NewBusinessError(e.FAILURE, "生成Token失败")
	}

	// 计算执行时间
	logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())

	// 记录刷新token日志
	if err := s.recordLoginLog(adminUserModel, claims, accessToken, logInfo, model.LoginTypeRefresh); err != nil {
		log.Logger.Error("记录刷新token日志失败", zap.Error(err), zap.Uint("user_id", id))
		// 记录日志失败不影响token刷新，继续返回token
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: "",
		TokenType:    tokenTypeBearer,
		ExpiresAt:    claims.ExpiresAt.Unix(),
	}, nil
}

// newAdminCustomClaims 创建管理员自定义Claims
func (s *LoginService) newAdminCustomClaims(user *model.AdminUser) token.AdminCustomClaims {
	return token.NewAdminCustomClaims(user)
}

// Logout 退出登录
func (s *LoginService) Logout(accessToken string) error {
	claims, err := s.parseToken(accessToken)
	if err != nil {
		return err
	}

	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return err
	}

	// 将Token加入Redis黑名单
	remainingTime := time.Until(exp.Time)
	blacklistKey := s.getBlacklistKey(claims.ID)
	if err := data.RedisClient().Set(context.Background(), blacklistKey, "1", remainingTime).Err(); err != nil {
		return err
	}

	// 更新登录日志表，标记token被撤销
	return s.revokeTokenInLog(claims.ID, model.RevokedCodeUserLogout, "用户主动登出（退出登录）")
}

// parseToken 解析Token
func (s *LoginService) parseToken(accessToken string) (*token.AdminCustomClaims, error) {
	claims := new(token.AdminCustomClaims)
	err := token.Parse(
		accessToken,
		claims,
		jwt.WithSubject(global.PcAdminSubject),
		jwt.WithIssuer(global.Issuer),
	)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// IsInBlacklist 判断Token是否在黑名单中
func (s *LoginService) IsInBlacklist(jwtId string) bool {
	result, err := data.RedisClient().
		Exists(context.Background(), s.getBlacklistKey(jwtId)).
		Result()
	if err != nil {
		return false
	}
	return result > 0
}

// getBlacklistKey 获取Redis黑名单的key
func (s *LoginService) getBlacklistKey(jwtId string) string {
	return blacklistPrefix + jwtId
}

// isTokenRevokedInLog 检查登录日志表中token是否被撤销
func (s *LoginService) isTokenRevokedInLog(jwtId string) bool {
	if jwtId == "" {
		return false
	}

	loginLog := model.NewAdminLoginLogs()
	err := loginLog.DB().Where("jwt_id = ? AND deleted_at = 0", jwtId).
		Select("is_revoked").
		First(loginLog).Error

	if err != nil {
		// 如果查询出错或记录不存在，返回false（允许通过，避免误判）
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Logger.Error("检查token撤销状态失败", zap.Error(err), zap.String("jwt_id", jwtId))
		}
		return false
	}

	return loginLog.IsRevoked == model.IsRevokedYes
}

// revokeTokenInLog 在登录日志表中标记token被撤销
func (s *LoginService) revokeTokenInLog(jwtId string, revokedCode uint8, revokedReason string) error {
	if jwtId == "" {
		return nil
	}

	loginLog := model.NewAdminLoginLogs()
	now := time.Now()
	revokedAt := utils.FormatDate{Time: now}

	// 更新登录日志表，标记token被撤销
	updates := map[string]interface{}{
		"is_revoked":     model.IsRevokedYes,
		"revoked_code":   revokedCode,
		"revoked_reason": revokedReason,
		"revoked_at":     revokedAt,
		"updated_at":     now,
	}

	err := loginLog.DB(loginLog).Where("jwt_id = ? AND deleted_at = 0 AND is_revoked = ?", jwtId, model.IsRevokedNo).
		Updates(updates).Error

	if err != nil {
		log.Logger.Error("更新登录日志token撤销状态失败", zap.Error(err), zap.String("jwt_id", jwtId))
		return err
	}

	return nil
}

// RevokeUserTokens 撤销用户所有未过期的token（用于禁用用户时）
// tx 可选参数，如果提供则使用事务连接，否则使用默认连接
func (s *LoginService) RevokeUserTokens(userId uint, revokedCode uint8, revokedReason string, tx ...*gorm.DB) error {
	if userId == 0 {
		return nil
	}

	loginLog := model.NewAdminLoginLogs()
	now := time.Now()

	// 如果提供了事务连接，使用事务连接；否则使用默认连接
	var db *gorm.DB
	if len(tx) > 0 && tx[0] != nil {
		db = tx[0]
		loginLog.SetDB(tx[0])
	} else {
		db = loginLog.DB()
	}

	// 查询该用户所有登录成功、未过期且未撤销的token
	// 只撤销登录成功的记录（login_status = 1），且 token 未过期（token_expires > now）
	var loginLogs []model.AdminLoginLogs
	err := db.Where("uid = ? AND deleted_at = 0 AND is_revoked = ? AND login_status = ? AND token_expires IS NOT NULL AND token_expires > ?",
		userId, model.IsRevokedNo, model.LoginStatusSuccess, now).
		Find(&loginLogs).Error

	if err != nil {
		log.Logger.Error("查询用户未过期token失败", zap.Error(err), zap.Uint("user_id", userId))
		return err
	}

	if len(loginLogs) == 0 {
		return nil // 没有未过期的token，直接返回
	}

	// 批量撤销这些token
	revokedAt := utils.FormatDate{Time: now}
	updates := map[string]interface{}{
		"is_revoked":     model.IsRevokedYes,
		"revoked_code":   revokedCode,
		"revoked_reason": revokedReason,
		"revoked_at":     revokedAt,
		"updated_at":     now,
	}

	// 收集所有需要撤销的jwt_id
	jwtIds := make([]string, 0, len(loginLogs))
	for _, loginLogItem := range loginLogs {
		if loginLogItem.JwtID != "" {
			jwtIds = append(jwtIds, loginLogItem.JwtID)
		}
	}

	if len(jwtIds) == 0 {
		return nil
	}

	// 更新登录日志表（使用事务连接）
	err = db.Model(&model.AdminLoginLogs{}).Where("jwt_id IN ? AND deleted_at = 0 AND is_revoked = ?", jwtIds, model.IsRevokedNo).
		Updates(updates).Error

	if err != nil {
		log.Logger.Error("批量撤销用户token失败", zap.Error(err), zap.Uint("user_id", userId), zap.Int("count", len(jwtIds)))
		return err
	}

	// 将这些token加入Redis黑名单
	ctx := context.Background()
	for _, loginLogItem := range loginLogs {
		if loginLogItem.JwtID == "" {
			continue
		}

		// 计算剩余过期时间
		var remainingTime time.Duration
		if loginLogItem.TokenExpires != nil {
			remainingTime = time.Until(loginLogItem.TokenExpires.Time)
			if remainingTime > 0 {
				blacklistKey := s.getBlacklistKey(loginLogItem.JwtID)
				if err := data.RedisClient().Set(ctx, blacklistKey, "1", remainingTime).Err(); err != nil {
					log.Logger.Error("将token加入Redis黑名单失败", zap.Error(err), zap.String("jwt_id", loginLogItem.JwtID))
					// 不阻断流程，继续处理其他token
				}
			}
		} else {
			// token_expires 为 NULL 的情况，设置一个默认过期时间（24小时）
			blacklistKey := s.getBlacklistKey(loginLogItem.JwtID)
			if err := data.RedisClient().Set(ctx, blacklistKey, "1", 24*time.Hour).Err(); err != nil {
				log.Logger.Error("将token加入Redis黑名单失败", zap.Error(err), zap.String("jwt_id", loginLogItem.JwtID))
			}
		}
	}

	return nil
}

// CheckToken 检查Token是否有效
func (s *LoginService) CheckToken(accessToken string) (*model.AdminUser, bool) {
	claims, err := s.parseToken(accessToken)
	if err != nil {
		return nil, false
	}

	// 检查Token过期时间
	if !s.isTokenValid(claims) {
		return nil, false
	}

	// 尝试自动刷新Token
	s.tryRefreshToken(claims)

	// 获取用户信息
	adminUser, err := s.getUserById(claims.UserID)
	if err != nil {
		return nil, false
	}

	// 检查用户状态和黑名单
	if !s.isUserValid(adminUser, claims.ID) {
		return adminUser, false
	}

	return adminUser, true
}

// isTokenValid 检查Token是否有效（未过期）
func (s *LoginService) isTokenValid(claims *token.AdminCustomClaims) bool {
	exp, err := claims.GetExpirationTime()
	return err == nil && exp != nil
}

// buildRefreshLogInfo 构建刷新token日志信息
func (s *LoginService) buildRefreshLogInfo(c *gin.Context) LoginLogInfo {
	userAgentStr := c.Request.UserAgent()
	ua := useragent.New(userAgentStr)
	os := ua.OS()
	browser, _ := ua.Browser()

	return LoginLogInfo{
		IP:        c.ClientIP(),
		UserAgent: userAgentStr,
		OS:        os,
		Browser:   browser,
	}
}

// tryRefreshToken 尝试自动刷新Token
func (s *LoginService) tryRefreshToken(claims *token.AdminCustomClaims) {
	if config.Config.Jwt.RefreshTTL <= 0 {
		return
	}

	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return
	}

	now := time.Now()
	diff := exp.Time.Sub(now)
	refreshTTL := config.Config.Jwt.RefreshTTL * time.Second

	if diff < refreshTTL && s.GetCtx() != nil {
		// 构建登录日志信息（用于记录刷新token日志）
		logInfo := s.buildRefreshLogInfo(s.GetCtx())
		tokenResponse, _ := s.Refresh(claims.UserID, logInfo)
		if tokenResponse != nil {
			s.GetCtx().Writer.Header().Set("refresh-access-token", tokenResponse.AccessToken)
			s.GetCtx().Writer.Header().Set("refresh-exp", strconv.FormatInt(tokenResponse.ExpiresAt, 10))
		}
	}
}

// getUserById 根据ID获取用户信息
func (s *LoginService) getUserById(userId uint) (*model.AdminUser, error) {
	adminUser := model.NewAdminUsers()
	err := adminUser.GetById(adminUser, userId)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("权限中间件获取用户信息失败", zap.Error(err))
	}
	return adminUser, err
}

// isUserValid 检查用户是否有效（状态和黑名单）
func (s *LoginService) isUserValid(adminUser *model.AdminUser, jwtId string) bool {
	// 检查用户状态
	if adminUser.Status != int8(model.AdminUserStatusEnabled) {
		return false
	}

	// 先检查是否在黑名单中（Redis检查更快）
	if s.IsInBlacklist(jwtId) {
		return false
	}

	// 再检查登录日志表中token是否被撤销
	if s.isTokenRevokedInLog(jwtId) {
		return false
	}

	return true
}
