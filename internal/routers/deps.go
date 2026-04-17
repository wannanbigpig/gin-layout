package routers

import (
	"sync"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	admin_v1 "github.com/wannanbigpig/gin-layout/internal/controller/admin_v1"
)

// ControllerDeps 控制器依赖容器。
// 所有控制器在此集中注册，便于单元测试替换和灰度切换。
type ControllerDeps struct {
	Demo       *controller.DemoController
	Login      *admin_v1.LoginController
	Common     *admin_v1.CommonController
	AdminUser  *admin_v1.AdminUserController
	Api        *admin_v1.ApiController
	Menu       *admin_v1.MenuController
	Role       *admin_v1.RoleController
	Dept       *admin_v1.DeptController
	RequestLog *admin_v1.RequestLogController
	LoginLog   *admin_v1.AdminLoginLogController
}

var defaultDepsOnce sync.Once
var defaultDeps *ControllerDeps

// DefaultControllerDeps 返回默认控制器依赖（生产环境使用）。
// 使用 sync.Once 确保只初始化一次，提升性能。
func DefaultControllerDeps() *ControllerDeps {
	defaultDepsOnce.Do(func() {
		defaultDeps = &ControllerDeps{
			Demo:       controller.NewDemoController(),
			Login:      admin_v1.NewLoginController(),
			Common:     admin_v1.NewCommonController(),
			AdminUser:  admin_v1.NewAdminUserController(),
			Api:        admin_v1.NewApiController(),
			Menu:       admin_v1.NewMenuController(),
			Role:       admin_v1.NewRoleController(),
			Dept:       admin_v1.NewDeptController(),
			RequestLog: admin_v1.NewRequestLogController(),
			LoginLog:   admin_v1.NewAdminLoginLogController(),
		}
	})
	return defaultDeps
}

// MockControllerDeps 返回测试用控制器依赖（可传入 mock 实现）。
func MockControllerDeps(deps *ControllerDeps) *ControllerDeps {
	defaultDeps := DefaultControllerDeps()
	if deps == nil {
		return defaultDeps
	}
	if deps.Demo != nil {
		defaultDeps.Demo = deps.Demo
	}
	if deps.Login != nil {
		defaultDeps.Login = deps.Login
	}
	if deps.Common != nil {
		defaultDeps.Common = deps.Common
	}
	if deps.AdminUser != nil {
		defaultDeps.AdminUser = deps.AdminUser
	}
	if deps.Api != nil {
		defaultDeps.Api = deps.Api
	}
	if deps.Menu != nil {
		defaultDeps.Menu = deps.Menu
	}
	if deps.Role != nil {
		defaultDeps.Role = deps.Role
	}
	if deps.Dept != nil {
		defaultDeps.Dept = deps.Dept
	}
	if deps.RequestLog != nil {
		defaultDeps.RequestLog = deps.RequestLog
	}
	if deps.LoginLog != nil {
		defaultDeps.LoginLog = deps.LoginLog
	}
	return defaultDeps
}
