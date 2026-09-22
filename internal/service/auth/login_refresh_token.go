package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"uuid"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mssola/useragent"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	tokenutil "github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
)

const defaultRefreshCookiePath = "/admin/v1/auth"

// BuildLoginLogInfo 从请求上下文构建登录日志信息。
func (s *LoginService) BuildLoginLogInfo(c *gin.Context) LoginLogInfo {
	userAgentStr := c.Request.UserAgent()
	os := unknownUserAgentPart
	browser := unknownUserAgentPart
	if userAgentStr != "" {
		ua := useragent.New(userAgentStr)
		os = normalizeUserAgentPart(ua.OS())
		browserValue, _ := ua.Browser()
		browser = normalizeUserAgentPart(browserValue)
	}

	return LoginLogInfo{
		IP:        c.ClientIP(),
		UserAgent: userAgentStr,
		OS:        os,
		Browser:   browser,
	}
}

const unknownUserAgentPart = "Unknown"

func normalizeUserAgentPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return unknownUserAgentPart
	}
	return value
}

func (s *LoginService) issueRefreshToken(user *model.AdminUser, accessClaims tokenutil.AdminCustomClaims, accessToken string, logInfo LoginLogInfo, logType uint8) (*model.AdminLoginLogs, string, error) {
	refreshToken, refreshClaims, err := s.generateRefreshToken(user)
	if err != nil {
		return nil, "", err
	}
	loginLog := s.buildLoginLog(user.ID, user.Username, accessClaims.ID, accessToken, refreshToken, accessClaims.ExpiresAt.Time, logInfo, model.LoginStatusSuccess, "", logType)
	loginLog.RefreshExpires = &utils.FormatDate{Time: refreshClaims.ExpiresAt.Time}
	if s.loginLogDB != nil {
		loginLog.SetDB(s.loginLogDB)
	}
	return loginLog, refreshToken, nil
}

func (s *LoginService) generateRefreshToken(user *model.AdminUser) (string, RefreshClaims, error) {
	cfg := s.currentConfig()
	now := time.Now().UTC()
	refreshTTL := cfg.Jwt.RefreshTokenTTL
	if refreshTTL <= 0 {
		refreshTTL = int((7 * 24 * time.Hour).Seconds())
	}
	expiresAt := now.Add(time.Duration(refreshTTL) * time.Second)
	claims := RefreshClaims{
		UserID:   user.ID,
		TokenID:  uuid.New().String(),
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    global.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   global.PcAdminRefreshSubject,
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}
	refreshToken, err := tokenutil.Generate(claims)
	return refreshToken, claims, err
}

func (s *LoginService) parseRefreshToken(refreshToken string) (*RefreshClaims, error) {
	claims := new(RefreshClaims)
	secret := []byte(s.currentConfig().Jwt.SecretKey)
	parsedToken, err := jwt.ParseWithClaims(refreshToken, claims, func(jwtToken *jwt.Token) (interface{}, error) {
		if _, ok := jwtToken.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", jwtToken.Header["alg"])
		}
		return secret, nil
	}, jwt.WithSubject(global.PcAdminRefreshSubject), jwt.WithIssuer(global.Issuer))
	if err != nil {
		return nil, err
	}
	if !parsedToken.Valid {
		return nil, e.NewBusinessError(e.RefreshTokenInvalid)
	}
	return claims, nil
}

func (s *LoginService) RefreshByToken(refreshToken string, logInfo LoginLogInfo) (*TokenResponse, string, error) {
	startTime := time.Now()
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, "", e.NewBusinessError(e.RefreshTokenInvalid)
	}
	refreshClaims, err := s.parseRefreshToken(refreshToken)
	if err != nil {
		return nil, "", e.NewBusinessError(e.RefreshTokenInvalid)
	}
	now := time.Now().UTC()

	loginLog := model.NewAdminLoginLogs()
	db := s.loginLogDB
	if db == nil {
		db, err = loginLog.GetDB()
		if err != nil {
			return nil, "", err
		}
	}

	oldRefreshHash := s.calculateTokenHash(refreshToken)
	var claims tokenutil.AdminCustomClaims
	var accessToken, newRefreshToken string
	err = db.Transaction(func(tx *gorm.DB) error {
		// 与账号安全变更统一使用 user -> session 的加锁顺序，确保禁用、改密、
		// 超级身份变更和 refresh 轮换之间不会产生提交后的新活跃会话。
		adminUserModel, err := loadAdminUserForAuthentication(tx, refreshClaims.UserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return e.NewBusinessError(e.RefreshTokenInvalid)
			}
			return err
		}
		if adminUserModel.Status != model.AdminUserStatusEnabled {
			return e.NewBusinessError(e.RefreshTokenInvalid)
		}
		current := model.NewAdminLoginLogs()
		current.SetDB(tx)
		if err := current.FindActiveByRefreshTokenHashForUpdate(oldRefreshHash, now); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return e.NewBusinessError(e.RefreshTokenInvalid)
			}
			return err
		}
		if current.UID != adminUserModel.ID {
			return e.NewBusinessError(e.RefreshTokenInvalid)
		}

		claims = s.newAdminCustomClaims(adminUserModel)
		accessToken, err = s.generateAccessToken(claims)
		if err != nil {
			return e.NewBusinessError(e.TokenGenerateFailed)
		}
		logInfo.ExecutionTime = int(time.Since(startTime).Milliseconds())
		newLog, issuedRefreshToken, issueErr := s.issueRefreshToken(adminUserModel, claims, accessToken, logInfo, model.LoginTypeRefresh)
		if issueErr != nil {
			return e.NewBusinessError(e.TokenGenerateFailed)
		}
		newRefreshToken = issuedRefreshToken

		revokedAt := utils.FormatDate{Time: now}
		if rows, err := current.RevokeActiveByRefreshTokenHash(oldRefreshHash, model.RevokedCodeTokenRefresh, "refresh token 轮换", revokedAt); err != nil {
			return err
		} else if rows == 0 {
			return e.NewBusinessError(e.RefreshTokenInvalid)
		}

		newLog.SetDB(tx)
		if err := newLog.Create(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	tokenResponse := &TokenResponse{
		AccessToken: accessToken,
		TokenType:   tokenTypeBearer,
		ExpiresAt:   claims.ExpiresAt.Unix(),
	}
	return tokenResponse, newRefreshToken, nil
}

func (s *LoginService) RefreshTokenFromCookie(c *gin.Context) (string, error) {
	cfg := s.currentConfig().Jwt
	cookie, err := c.Cookie(refreshCookieName(cfg.RefreshCookieName))
	if err != nil {
		return "", err
	}
	return cookie, nil
}

func (s *LoginService) SetRefreshCookie(c *gin.Context, refreshToken string) {
	cfg := s.currentConfig().Jwt
	maxAge := int(cfg.RefreshTokenTTL)
	if maxAge <= 0 {
		maxAge = int((7 * 24 * time.Hour).Seconds())
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookieName(cfg.RefreshCookieName),
		Value:    refreshToken,
		Path:     refreshCookiePath(cfg.RefreshCookiePath),
		Domain:   cfg.RefreshCookieDomain,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   cfg.RefreshCookieSecure,
		SameSite: refreshCookieSameSite(cfg.RefreshCookieSameSite),
	})
}

func (s *LoginService) ClearRefreshCookie(c *gin.Context) {
	cfg := s.currentConfig().Jwt
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookieName(cfg.RefreshCookieName),
		Value:    "",
		Path:     refreshCookiePath(cfg.RefreshCookiePath),
		Domain:   cfg.RefreshCookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.RefreshCookieSecure,
		SameSite: refreshCookieSameSite(cfg.RefreshCookieSameSite),
	})
}

func refreshCookieName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "refresh_token"
	}
	return name
}

func refreshCookiePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return defaultRefreshCookiePath
	}
	return path
}

func refreshCookieSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax", "":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteLaxMode
	}
}
