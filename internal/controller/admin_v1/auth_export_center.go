package admin_v1

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
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

// Summary 汇总当前用户的导出状态。
func (api ExportCenterController) Summary(c *gin.Context) {
	result, err := service.NewExportCenterService().MySummary(c.Request.Context(), api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// List 分页查询导出记录列表。
func (api ExportCenterController) List(c *gin.Context) {
	params := form.NewExportRecordListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	svc := service.NewExportCenterService()
	userID := api.GetCurrentUserID(c)
	var result *resources.Collection
	var summary *resources.MyExportSummaryResources
	var summaryErr error
	var group errgroup.Group
	group.Go(func() error {
		var err error
		result, err = svc.MyList(c.Request.Context(), userID, params)
		return err
	})
	group.Go(func() error {
		summary, summaryErr = svc.MySummary(c.Request.Context(), userID)
		return nil
	})
	if err := group.Wait(); err != nil {
		api.Err(c, err)
		return
	}
	if summaryErr != nil {
		log.Logger.Error("export list summary failed", zap.Uint("user_id", userID), zap.Error(summaryErr))
	}
	api.Success(c, &resources.MyExportRecordListResources{Collection: result, Summary: summary})
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
