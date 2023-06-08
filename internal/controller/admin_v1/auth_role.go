package admin_v1

import "github.com/wannanbigpig/gin-layout/internal/controller"

type RoleController struct {
	controller.Api
}

func NewRoleController() *RoleController {
	return &RoleController{}
}
