package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service/dashboard"
)

type DashboardController struct {
	controller.Api
}

type personalDashboardService interface {
	SetAdminUserId(uint)
	Personal() (*dashboard.Personal, error)
}

var newPersonalDashboardService = func() personalDashboardService {
	return dashboard.NewOverviewService()
}

func NewDashboardController() *DashboardController {
	return &DashboardController{}
}

// Personal 返回当前登录用户自己的登录摘要与最近操作。
func (api DashboardController) Personal(c *gin.Context) {
	service := newPersonalDashboardService()
	service.SetAdminUserId(api.GetCurrentUserID(c))

	personal, err := service.Personal()
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, personal)
}

func (api DashboardController) Overview(c *gin.Context) {
	service := dashboard.NewOverviewService()
	service.SetAdminUserId(api.GetCurrentUserID(c))

	overview, err := service.Overview()
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, overview)
}

func (api DashboardController) Statistics(c *gin.Context) {
	service := dashboard.NewOverviewService()
	service.SetAdminUserId(api.GetCurrentUserID(c))

	statistics, err := service.Statistics()
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, statistics)
}

func (api DashboardController) Monitor(c *gin.Context) {
	service := dashboard.NewMonitorService()

	snapshot, err := service.GetSnapshot()
	if err != nil {
		api.Err(c, err)
		return
	}

	api.Success(c, snapshot)
}
