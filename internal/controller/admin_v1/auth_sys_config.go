package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	"github.com/wannanbigpig/gin-layout/internal/service/sys_config"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// SysConfigController 系统参数控制器。
type SysConfigController struct {
	controller.Api
}

func NewSysConfigController() *SysConfigController {
	return &SysConfigController{}
}

func (api SysConfigController) List(c *gin.Context) {
	params := form.NewSysConfigListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	api.Success(c, sys_config.NewSysConfigService().List(params, middleware.LocaleFromContext(c)))
}

func (api SysConfigController) Detail(c *gin.Context) {
	params := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	detail, err := sys_config.NewSysConfigService().Detail(params.ID, middleware.LocaleFromContext(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, detail)
}

func (api SysConfigController) Value(c *gin.Context) {
	params := form.NewSysConfigKeyQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	value, err := sys_config.NewSysConfigService().Value(params.ConfigKey)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, value)
}

func (api SysConfigController) Create(c *gin.Context) {
	params := form.NewCreateSysConfigForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	changeDiff, err := sys_config.NewSysConfigService().CreateWithAuditDiff(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

func (api SysConfigController) Update(c *gin.Context) {
	params := form.NewUpdateSysConfigForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	changeDiff, err := sys_config.NewSysConfigService().UpdateWithAuditDiff(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

func (api SysConfigController) Delete(c *gin.Context) {
	params := form.NewIdForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	changeDiff, err := sys_config.NewSysConfigService().DeleteWithAuditDiff(params.ID)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

func (api SysConfigController) Refresh(c *gin.Context) {
	if err := sys_config.NewSysConfigService().RefreshCache(); err != nil {
		api.Err(c, err)
		return
	}
	diff := auditdiff.Marshal(auditdiff.BuildFieldDiff(nil, map[string]any{
		"action": "refresh_cache",
	}, []auditdiff.FieldRule{
		{Field: "action", Label: "操作"},
	}))
	middleware.SetAuditChangeDiffRaw(c, diff)
	api.Success(c, nil)
}
