package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
	"github.com/wannanbigpig/gin-layout/internal/service/permission"
)

func AdminAuthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := c.GetHeader("Authorization")
		accessToken, err := token.GetAccessToken(authorization)
		if err != nil {
			response.Fail(c, e.NotLogin, err.Error())
			return
		}
		adminCustomClaims := new(token.AdminCustomClaims)
		// 解析Token
		err = token.Parse(accessToken, adminCustomClaims, jwt.WithSubject(global.PcAdminSubject), jwt.WithIssuer(global.Issuer))
		if err != nil || adminCustomClaims == nil {
			response.FailCode(c, e.NotLogin)
			return
		}

		exp, err := adminCustomClaims.GetExpirationTime()
		// 获取过期时间返回err,或者exp为nil都返回错误
		if err != nil || exp == nil {
			response.FailCode(c, e.NotLogin)
			return
		}

		// 刷新时间大于0则判断剩余时间小于刷新时间时刷新Token并在Response header中返回
		if config.Config.Jwt.RefreshTTL > 0 {
			now := time.Now()
			diff := exp.Time.Sub(now)
			refreshTTL := config.Config.Jwt.RefreshTTL * time.Second
			if diff < refreshTTL {
				tokenResponse, _ := permission.NewLoginService().Refresh(adminCustomClaims.UserID)
				c.Writer.Header().Set("refresh-access-token", tokenResponse.AccessToken)
				c.Writer.Header().Set("refresh-exp", strconv.FormatInt(tokenResponse.ExpiresAt, 10))
			}
		}

		// 检测是否在黑名单中
		if permission.NewLoginService().IsInBlacklist(adminCustomClaims.ID) {
			response.FailCode(c, e.NotLogin)
			return
		}

		c.Set("uid", adminCustomClaims.UserID)
		c.Set("full_phone_number", adminCustomClaims.FullPhoneNumber)
		c.Set("nickname", adminCustomClaims.Nickname)
		c.Set("email", adminCustomClaims.Email)
		c.Next()
	}
}
