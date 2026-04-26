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
	SysConfig  *admin_v1.SysConfigController
	SysDict    *admin_v1.SysDictController
	RequestLog *admin_v1.RequestLogController
	LoginLog   *admin_v1.AdminLoginLogController
	TaskCenter *admin_v1.TaskCenterController
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
			SysConfig:  admin_v1.NewSysConfigController(),
			SysDict:    admin_v1.NewSysDictController(),
			RequestLog: admin_v1.NewRequestLogController(),
			LoginLog:   admin_v1.NewAdminLoginLogController(),
			TaskCenter: admin_v1.NewTaskCenterController(),
		}
	})
	return defaultDeps
}

func normalizeControllerDeps(deps *ControllerDeps) *ControllerDeps {
	defaultDeps := DefaultControllerDeps()
	if deps == nil {
		return defaultDeps
	}
	if deps.Demo == nil {
		deps.Demo = defaultDeps.Demo
	}
	if deps.Login == nil {
		deps.Login = defaultDeps.Login
	}
	if deps.Common == nil {
		deps.Common = defaultDeps.Common
	}
	if deps.AdminUser == nil {
		deps.AdminUser = defaultDeps.AdminUser
	}
	if deps.Api == nil {
		deps.Api = defaultDeps.Api
	}
	if deps.Menu == nil {
		deps.Menu = defaultDeps.Menu
	}
	if deps.Role == nil {
		deps.Role = defaultDeps.Role
	}
	if deps.Dept == nil {
		deps.Dept = defaultDeps.Dept
	}
	if deps.SysConfig == nil {
		deps.SysConfig = defaultDeps.SysConfig
	}
	if deps.SysDict == nil {
		deps.SysDict = defaultDeps.SysDict
	}
	if deps.RequestLog == nil {
		deps.RequestLog = defaultDeps.RequestLog
	}
	if deps.LoginLog == nil {
		deps.LoginLog = defaultDeps.LoginLog
	}
	if deps.TaskCenter == nil {
		deps.TaskCenter = defaultDeps.TaskCenter
	}
	return deps
}

// MockControllerDeps 返回测试用控制器依赖（可传入 mock 实现）。
func MockControllerDeps(deps *ControllerDeps) *ControllerDeps {
	if deps == nil {
		return DefaultControllerDeps()
	}
	return normalizeControllerDeps(deps)
}
