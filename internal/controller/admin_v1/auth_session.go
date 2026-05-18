package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// SessionController 在线会话管理控制器。
type SessionController struct {
	controller.Api
}

func NewSessionController() *SessionController {
	return &SessionController{}
}

// List 分页查询在线会话列表。
func (api SessionController) List(c *gin.Context) {
	params := form.NewSessionListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result := auth.NewLoginService().ListSessions(params)
	api.Success(c, result)
}

// Revoke 撤销在线会话。
func (api SessionController) Revoke(c *gin.Context) {
	params := form.NewSessionRevokeForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	if err := auth.NewLoginService().RevokeSession(c.Request.Context(), params.ID, params.Reason); err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}
