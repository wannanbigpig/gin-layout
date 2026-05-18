package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service/dashboard"
)

type DashboardController struct {
	controller.Api
}

func NewDashboardController() *DashboardController {
	return &DashboardController{}
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
