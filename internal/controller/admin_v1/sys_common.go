package admin_v1

import (
	"github.com/wannanbigpig/gin-layout/internal/controller"
)

type CommonController struct {
	controller.Api
}

func NewCommonController() *CommonController {
	return &CommonController{}
}
