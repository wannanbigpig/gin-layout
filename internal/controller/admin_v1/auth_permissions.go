package admin_v1

import "github.com/wannanbigpig/gin-layout/internal/controller"

type PermissionsController struct {
	controller.Api
}

func NewPermissionsController() *PermissionsController {
	return &PermissionsController{}
}
