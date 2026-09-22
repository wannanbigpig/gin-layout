package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type SMSConfigController struct{ controller.Api }

func NewSMSConfigController() *SMSConfigController { return &SMSConfigController{} }

func (api SMSConfigController) Config(c *gin.Context) {
	settings, err := service.NewSMSConfigService().Get(true)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, settings)
}

func (api SMSConfigController) Save(c *gin.Context) {
	params := form.NewSMSConfigPayload()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	diff, err := service.NewSMSConfigService().Save(params.ToConfig())
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, diff)
	api.Success(c, nil)
}

func (api SMSConfigController) Test(c *gin.Context) {
	params := form.NewSMSTestPayload()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	if err := service.NewSMSConfigService().Test(
		c.Request.Context(), c.GetUint(global.ContextKeyUID), params.CountryCode, params.PhoneNumber, params.Config.ToConfig(),
	); err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}
