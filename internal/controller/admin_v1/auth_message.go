package admin_v1

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
	"github.com/wannanbigpig/gin-layout/internal/service/notification"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// SysMessageController 提供最小可用消息接口。
type SysMessageController struct {
	controller.Api
}

func NewSysMessageController() *SysMessageController {
	return &SysMessageController{}
}

func (api SysMessageController) List(c *gin.Context) {
	params := form.NewSysMessageListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	result, err := notification.NewService().List(api.GetCurrentUserID(c), params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

func (api SysMessageController) UnreadCount(c *gin.Context) {
	count, err := notification.NewService().UnreadCount(c.Request.Context(), api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, map[string]any{"count": count})
}

func (api SysMessageController) Read(c *gin.Context) {
	params := form.NewSysMessageReadForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	updated, unreadCount, err := notification.NewService().MarkRead(c.Request.Context(), api.GetCurrentUserID(c), params.ID)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, map[string]any{"updated": updated, "unread_count": unreadCount})
}

func (api SysMessageController) ReadAll(c *gin.Context) {
	params := form.NewSysMessageReadAllForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	affected, unreadCount, err := notification.NewService().MarkAllRead(c.Request.Context(), api.GetCurrentUserID(c), params.Category)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, map[string]any{"affected": affected, "unread_count": unreadCount})
}

// WS 通过 query token 建立消息推送连接。
// 鉴权失败时直接返回 HTTP 401，避免 WebSocket 客户端拿到 200+JSON 后无法判断错误来源。
func (api SysMessageController) WS(c *gin.Context) {
	accessToken := strings.TrimSpace(c.Query("access_token"))
	if accessToken == "" {
		accessToken = strings.TrimSpace(c.Query("token"))
	}
	if accessToken == "" {
		if log.Logger != nil {
			log.Logger.Warn("notification ws: missing access_token", zap.String("remote_addr", c.ClientIP()))
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": e.NotLogin, "msg": "missing access_token"})
		return
	}
	principal, ok := auth.NewLoginService().ResolvePrincipal(accessToken)
	if !ok || principal == nil || principal.UserID == 0 {
		if log.Logger != nil {
			log.Logger.Warn("notification ws: resolve principal failed",
				zap.String("remote_addr", c.ClientIP()),
				zap.Int("token_len", len(accessToken)),
				zap.String("token_prefix", safeTokenPrefix(accessToken)),
			)
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": e.NotLogin, "msg": "invalid or expired token"})
		return
	}
	if err := notification.ServeWebsocket(c.Writer, c.Request, principal.UserID); err != nil {
		if !c.Writer.Written() {
			api.Fail(c, e.InvalidParameter, err.Error())
		}
	}
}

func safeTokenPrefix(token string) string {
	if len(token) <= 12 {
		return token
	}
	return token[:12] + "..."
}
