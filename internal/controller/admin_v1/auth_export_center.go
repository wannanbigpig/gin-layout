package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
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
	result, err := service.NewExportCenterService().MyList(c.Request.Context(), api.GetCurrentUserID(c), params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// Detail 查询导出记录详情。
func (api ExportCenterController) Detail(c *gin.Context) {
	query := form.MyExportRecordDetail{}
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}
	detail, err := service.NewExportCenterService().MyDetail(c.Request.Context(), api.GetCurrentUserID(c), query.PublicID)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, detail)
}
