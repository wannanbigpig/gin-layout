package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// ExportCenterController 统一导出中心控制器。
type ExportCenterController struct {
	controller.Api
}

func NewExportCenterController() *ExportCenterController {
	return &ExportCenterController{}
}

// List 分页查询导出记录列表。
func (api ExportCenterController) List(c *gin.Context) {
	params := form.NewExportRecordListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	uid := api.GetCurrentUserID(c)
	params.TriggerUserID = &uid
	api.Success(c, service.NewExportCenterService().List(params))
}

// Detail 查询导出记录详情。
func (api ExportCenterController) Detail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}
	detail, err := service.NewExportCenterService().Detail(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}
	record, ok := detail.(*resources.ExportRecordResources)
	if !ok || record.TriggerUserID != api.GetCurrentUserID(c) {
		api.FailCode(c, e.NotFound)
		return
	}
	api.Success(c, detail)
}
