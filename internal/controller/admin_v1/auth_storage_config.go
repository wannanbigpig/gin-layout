package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/sys_config"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"go.uber.org/zap"
)

type StorageConfigController struct {
	controller.Api
}

func NewStorageConfigController() *StorageConfigController {
	return &StorageConfigController{}
}

func (api StorageConfigController) Config(c *gin.Context) {
	settings, err := service.NewStorageConfigService().Get(true)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, settings)
}

func (api StorageConfigController) Save(c *gin.Context) {
	params := form.NewStorageConfigPayload()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	diff, err := service.NewStorageConfigService().SaveWithAuditDiff(service.StorageSettings{
		ActiveDriver: params.ActiveDriver,
		Config:       params.Config,
	})
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, diff)
	if err := sys_config.NewSysConfigService().RefreshCache(); err != nil {
		log.Logger.Warn("存储配置已保存，运行时缓存刷新失败", zap.Error(err))
	}
	api.Success(c, nil)
}

func (api StorageConfigController) Test(c *gin.Context) {
	params := form.NewStorageConfigPayload()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	if err := service.NewStorageConfigService().Test(c.Request.Context(), service.StorageSettings{
		ActiveDriver: params.ActiveDriver,
		Config:       params.Config,
	}); err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}

func (api StorageConfigController) Secret(c *gin.Context) {
	query := form.NewStorageSecretQuery()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	value, err := service.NewStorageConfigService().GetSensitiveValue(query.Driver, query.Field)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, map[string]string{
		"value": value,
	})
}
