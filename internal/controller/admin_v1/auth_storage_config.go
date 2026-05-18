package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/sys_config"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
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
	if err := service.NewStorageConfigService().Save(service.StorageSettings{
		ActiveDriver: params.ActiveDriver,
		Config:       params.Config,
	}); err != nil {
		api.Err(c, err)
		return
	}
	if err := sys_config.NewSysConfigService().RefreshCache(); err != nil {
		api.Err(c, err)
		return
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
