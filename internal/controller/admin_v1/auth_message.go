package admin_v1

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
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

// Send 管理员手动发送系统通知
func (api SysMessageController) Send(c *gin.Context) {
	params := form.NewSysMessageSendForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := notification.NewService().Send(c.Request.Context(), notification.SendInput{
		Title:        params.Title,
		Content:      params.Content,
		Category:     params.Category,
		ActionURL:    params.ActionURL,
		ActionLabel:  params.ActionLabel,
		AudienceType: params.AudienceType,
		UserIDs:      params.UserIDs,
		SenderID:     api.GetCurrentUserID(c),
	})
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
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

// GetWsTicket 生成 WebSocket 建立连接的临时票据
func (api SysMessageController) GetWsTicket(c *gin.Context) {
	principal := auth.GetAuthPrincipal(c)
	if principal == nil || principal.UserID == 0 {
		api.FailCode(c, e.NotLogin)
		return
	}

	ticket := uuid.New().String()
	redisKey := fmt.Sprintf("ws:ticket:%s", ticket)

	rdb := data.RedisClient()
	if rdb == nil {
		api.FailCode(c, e.ServiceDependencyNotReady)
		return
	}
	principalJSON, err := json.Marshal(principal)
	if err != nil {
		if log.Logger != nil {
			log.Logger.Error("websocket ticket: marshal principal failed",
				zap.Uint("user_id", principal.UserID),
				zap.Error(err),
			)
		}
		api.FailCode(c, e.ServerErr)
		return
	}
	if err := rdb.Set(c.Request.Context(), redisKey, principalJSON, 30*time.Second).Err(); err != nil {
		api.FailCode(c, e.ServerErr)
		return
	}

	api.Success(c, map[string]any{"ticket": ticket})
}

// WS 建立通用实时推送连接。支持 ticket 校验方式，并向下兼容 token/access_token。
// 鉴权失败时直接返回 HTTP 401，避免 WebSocket 客户端拿到 200+JSON 后无法判断错误来源。
func (api SysMessageController) WS(c *gin.Context) {
	var principal *auth.AuthPrincipal
	var ok bool

	ticket := strings.TrimSpace(c.Query("ticket"))
	if ticket != "" {
		redisKey := fmt.Sprintf("ws:ticket:%s", ticket)
		rdb := data.RedisClient()

		// 管道执行：读取并即刻删除 Ticket
		pipe := rdb.Pipeline()
		getCmd := pipe.Get(c.Request.Context(), redisKey)
		pipe.Del(c.Request.Context(), redisKey)
		_, err := pipe.Exec(c.Request.Context())

		if err == nil {
			var cachedPrincipal auth.AuthPrincipal
			if err := json.Unmarshal([]byte(getCmd.Val()), &cachedPrincipal); err == nil && cachedPrincipal.UserID != 0 {
				// 在 Redis 消费 Ticket 时，反查一次主登录 Session 的存活状态（黑名单与撤销检测）
				if auth.NewLoginService().IsPrincipalValid(cachedPrincipal.Claims) {
					principal = &cachedPrincipal
					ok = true
				}
			}
		}
	}

	// 兼容旧的 token 传参形式
	if !ok {
		accessToken := strings.TrimSpace(c.Query("access_token"))
		if accessToken == "" {
			accessToken = strings.TrimSpace(c.Query("token"))
		}
		if accessToken != "" {
			principal, ok = auth.NewLoginService().ResolvePrincipal(accessToken)
		}
	}

	if !ok || principal == nil || principal.UserID == 0 {
		if log.Logger != nil {
			log.Logger.Warn("websocket ws: authentication failed",
				zap.String("remote_addr", c.ClientIP()),
				zap.String("ticket", ticket),
			)
		}
		api.FailCode(c, e.NotLogin)
		return
	}

	// ServeWebsocket 阻塞至连接断开才返回：err == nil 表示成功建连并正常断开。
	// WS 经 Hijack 旁路了 CustomLogger，故在此主动补记一条审计日志，
	// 执行时长取请求开始至断开的连接全程时长（ExecutionTimeScopeConnection）。
	// 无论连接正常断开还是因读写错误断开，均记录审计日志（断开原因可通过 err 区分）。
	err := notification.ServeWebsocket(c.Writer, c.Request, principal.Claims)
	middleware.LogWebSocketConnection(c)
	if err != nil {
		if !c.Writer.Written() {
			api.Fail(c, e.InvalidParameter, err.Error())
		}
	}
}
