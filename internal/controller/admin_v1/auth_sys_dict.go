package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/service/sys_dict"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// SysDictController 系统字典控制器。
type SysDictController struct {
	controller.Api
}

func NewSysDictController() *SysDictController {
	return &SysDictController{}
}

func (api SysDictController) TypeList(c *gin.Context) {
	params := form.NewSysDictTypeListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	api.Success(c, sys_dict.NewSysDictService().TypeList(params, middleware.LocaleFromContext(c)))
}

func (api SysDictController) TypeDetail(c *gin.Context) {
	params := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	detail, err := sys_dict.NewSysDictService().TypeDetail(params.ID, middleware.LocaleFromContext(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, detail)
}

func (api SysDictController) TypeCreate(c *gin.Context) {
	params := form.NewCreateSysDictTypeForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	changeDiff, err := sys_dict.NewSysDictService().CreateTypeWithAuditDiff(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

func (api SysDictController) TypeUpdate(c *gin.Context) {
	params := form.NewUpdateSysDictTypeForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	changeDiff, err := sys_dict.NewSysDictService().UpdateTypeWithAuditDiff(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

func (api SysDictController) TypeDelete(c *gin.Context) {
	params := form.NewIdForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	changeDiff, err := sys_dict.NewSysDictService().DeleteTypeWithAuditDiff(params.ID)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

func (api SysDictController) ItemList(c *gin.Context) {
	params := form.NewSysDictItemListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	api.Success(c, sys_dict.NewSysDictService().ItemList(params, middleware.LocaleFromContext(c)))
}

func (api SysDictController) ItemCreate(c *gin.Context) {
	params := form.NewCreateSysDictItemForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	changeDiff, err := sys_dict.NewSysDictService().CreateItemWithAuditDiff(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

func (api SysDictController) ItemUpdate(c *gin.Context) {
	params := form.NewUpdateSysDictItemForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	changeDiff, err := sys_dict.NewSysDictService().UpdateItemWithAuditDiff(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

func (api SysDictController) ItemDelete(c *gin.Context) {
	params := form.NewIdForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	changeDiff, err := sys_dict.NewSysDictService().DeleteItemWithAuditDiff(params.ID)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}

func (api SysDictController) Options(c *gin.Context) {
	params := form.NewSysDictOptionsQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	options, err := sys_dict.NewSysDictService().Options(params.TypeCode, middleware.LocaleFromContext(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, options)
}
