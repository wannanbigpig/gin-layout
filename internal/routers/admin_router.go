package routers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/middleware"
)

// 路由构建辅助函数（减少重复代码）
func GET(path, title string, auth AuthMode, handlers ...gin.HandlerFunc) RouteDef {
	return RouteDef{Method: http.MethodGet, Path: path, Title: title, Auth: auth, Handlers: handlers}
}

func POST(path, title string, auth AuthMode, handlers ...gin.HandlerFunc) RouteDef {
	return RouteDef{Method: http.MethodPost, Path: path, Title: title, Auth: auth, Handlers: handlers}
}

// AdminRouteTree 返回管理员后台路由声明树。
// deps: 控制器依赖容器，传入 nil 则使用默认实现
func AdminRouteTree(deps *ControllerDeps) RouteGroupDef {
	deps = normalizeControllerDeps(deps)

	return RouteGroupDef{
		Prefix: "admin/v1",
		Children: []RouteGroupDef{
			adminOtherGroup(deps),
			adminAuthGroup(deps),
		},
	}
}

// adminOtherGroup 其他路由组（公开接口、登录等）
func adminOtherGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		GroupCode: "other",
		Routes: []RouteDef{
			GET("demo", "Demo 示例", AuthModeNone, deps.Demo.HelloWorld).WithDesc("Demo 示例备注"),
			GET("file/:uuid", "获取文件", AuthModeNone, middleware.DatabaseReadyGuard(), deps.Common.GetFile),
		},
		Children: []RouteGroupDef{
			{
				GroupCode: "login",
				Routes: []RouteDef{
					POST("login", "登录", AuthModeNone, middleware.DatabaseReadyGuard(), deps.Login.Login).WithDesc("用户登录接口"),
					GET("login-captcha", "验证码", AuthModeNone, deps.Login.LoginCaptcha).WithDesc("获取登录验证码"),
				},
			},
		},
	}
}

// adminAuthGroup 需要认证的路由组
func adminAuthGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Middleware: []gin.HandlerFunc{middleware.OptionalDatabaseReadyGuard(), middleware.AdminAuthHandler()},
		Children: []RouteGroupDef{
			commonGroup(deps),
			authGroup(deps),
			adminUserGroup(deps),
			permissionGroup(deps),
			menuGroup(deps),
			roleGroup(deps),
			deptGroup(deps),
			systemGroup(deps),
			logGroup(deps),
			taskGroup(deps),
		},
	}
}

// commonGroup 通用接口组
func commonGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "common",
		GroupCode: "common",
		Routes: []RouteDef{
			POST("upload", "上传文件", AuthModeLogin, deps.Common.Upload),
		},
	}
}

// authGroup 认证相关接口组
func authGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "auth",
		GroupCode: "auth",
		Routes: []RouteDef{
			POST("logout", "退出登录", AuthModeLogin, deps.Login.Logout),
			GET("check-token", "检查 Token", AuthModeLogin, deps.Login.CheckToken).WithDesc("验证 Token 有效性"),
		},
	}
}

// adminUserGroup 管理员用户管理组
func adminUserGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "admin-user",
		GroupCode: "adminUser",
		Routes: []RouteDef{
			// 个人资料（AuthModeLogin：只需登录）
			GET("get", "获取当前用户信息", AuthModeLogin, deps.AdminUser.GetUserInfo),
			GET("user-menu-info", "获取用户权限信息", AuthModeLogin, deps.AdminUser.GetUserMenuInfo),
			POST("update-profile", "更新个人资料", AuthModeLogin, deps.AdminUser.UpdateProfile),

			// 用户管理（AuthModeAuth：需要权限）
			GET("list", "管理员列表", AuthModeAuth, deps.AdminUser.List),
			GET("detail", "管理员详情", AuthModeAuth, deps.AdminUser.Detail),
			GET("get-full-phone", "获取完整手机号", AuthModeAuth, deps.AdminUser.GetFullPhone).WithDesc("脱敏前完整手机号"),
			GET("get-full-email", "获取完整邮箱", AuthModeAuth, deps.AdminUser.GetFullEmail).WithDesc("脱敏前完整邮箱"),
			POST("create", "新增管理员", AuthModeAuth, deps.AdminUser.Create),
			POST("update", "更新管理员", AuthModeAuth, deps.AdminUser.Update),
			POST("delete", "删除管理员", AuthModeAuth, deps.AdminUser.Delete),
			POST("bind-role", "绑定角色", AuthModeAuth, deps.AdminUser.BindRole),
		},
	}
}

// permissionGroup 接口权限管理组
func permissionGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "permission",
		GroupCode: "api",
		Routes: []RouteDef{
			POST("update", "更新接口", AuthModeAuth, deps.Api.Update),
			GET("list", "接口列表", AuthModeAuth, deps.Api.List),
		},
	}
}

// menuGroup 菜单管理组
func menuGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "menu",
		GroupCode: "menu",
		Routes: []RouteDef{
			GET("list", "菜单列表", AuthModeAuth, deps.Menu.List),
			POST("delete", "删除菜单", AuthModeAuth, deps.Menu.Delete),
			POST("create", "新增菜单", AuthModeAuth, deps.Menu.Create),
			POST("update", "更新菜单", AuthModeAuth, deps.Menu.Update),
			POST("update-all-menu-permissions", "刷新菜单权限缓存", AuthModeAuth, deps.Menu.UpdateAllMenuPermissions),
			GET("detail", "菜单详情", AuthModeAuth, deps.Menu.Detail),
		},
	}
}

// roleGroup 角色管理组
func roleGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "role",
		GroupCode: "role",
		Routes: []RouteDef{
			GET("list", "角色列表", AuthModeAuth, deps.Role.List),
			POST("create", "新增角色", AuthModeAuth, deps.Role.Create),
			POST("update", "更新角色", AuthModeAuth, deps.Role.Update),
			GET("detail", "角色详情", AuthModeAuth, deps.Role.Detail),
			POST("delete", "删除角色", AuthModeAuth, deps.Role.Delete),
		},
	}
}

// deptGroup 部门管理组
func deptGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "department",
		GroupCode: "department",
		Routes: []RouteDef{
			GET("list", "部门列表", AuthModeAuth, deps.Dept.List),
			POST("create", "新增部门", AuthModeAuth, deps.Dept.Create),
			POST("update", "更新部门", AuthModeAuth, deps.Dept.Update),
			GET("detail", "部门详情", AuthModeAuth, deps.Dept.Detail),
			POST("delete", "删除部门", AuthModeAuth, deps.Dept.Delete),
			POST("bind-role", "部门绑定角色", AuthModeAuth, deps.Dept.BindRole),
		},
	}
}

// systemGroup 系统管理组
func systemGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "system",
		GroupCode: "system",
		Children: []RouteGroupDef{
			{
				Prefix:    "config",
				GroupCode: "sysConfig",
				Routes: []RouteDef{
					GET("list", "系统参数列表", AuthModeAuth, deps.SysConfig.List),
					GET("detail", "系统参数详情", AuthModeAuth, deps.SysConfig.Detail),
					GET("value", "获取系统参数值", AuthModeAuth, deps.SysConfig.Value),
					POST("create", "新增系统参数", AuthModeAuth, deps.SysConfig.Create),
					POST("update", "更新系统参数", AuthModeAuth, deps.SysConfig.Update),
					POST("delete", "删除系统参数", AuthModeAuth, deps.SysConfig.Delete),
					POST("refresh", "刷新系统参数缓存", AuthModeAuth, deps.SysConfig.Refresh),
				},
			},
			{
				Prefix:    "dict",
				GroupCode: "sysDict",
				Routes: []RouteDef{
					GET("type/list", "字典类型列表", AuthModeAuth, deps.SysDict.TypeList),
					GET("type/detail", "字典类型详情", AuthModeAuth, deps.SysDict.TypeDetail),
					POST("type/create", "新增字典类型", AuthModeAuth, deps.SysDict.TypeCreate),
					POST("type/update", "更新字典类型", AuthModeAuth, deps.SysDict.TypeUpdate),
					POST("type/delete", "删除字典类型", AuthModeAuth, deps.SysDict.TypeDelete),
					GET("item/list", "字典项列表", AuthModeAuth, deps.SysDict.ItemList),
					POST("item/create", "新增字典项", AuthModeAuth, deps.SysDict.ItemCreate),
					POST("item/update", "更新字典项", AuthModeAuth, deps.SysDict.ItemUpdate),
					POST("item/delete", "删除字典项", AuthModeAuth, deps.SysDict.ItemDelete),
					GET("options", "字典选项", AuthModeAuth, deps.SysDict.Options),
				},
			},
		},
	}
}

// logGroup 日志管理组
func logGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "log",
		GroupCode: "log",
		Children: []RouteGroupDef{
			{
				Prefix: "request",
				Routes: []RouteDef{
					GET("list", "请求日志列表", AuthModeAuth, deps.RequestLog.List),
					GET("detail", "请求日志详情", AuthModeAuth, deps.RequestLog.Detail),
					GET("export", "导出请求日志", AuthModeAuth, deps.RequestLog.Export),
					GET("mask-config", "获取请求日志脱敏配置", AuthModeAuth, deps.RequestLog.MaskConfig),
					POST("mask-config", "更新请求日志脱敏配置", AuthModeAuth, deps.RequestLog.UpdateMaskConfig),
				},
			},
			{
				Prefix: "login",
				Routes: []RouteDef{
					GET("list", "登录日志列表", AuthModeAuth, deps.LoginLog.List),
					GET("detail", "登录日志详情", AuthModeAuth, deps.LoginLog.Detail),
				},
			},
		},
	}
}

// taskGroup 任务中心组
func taskGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "task",
		GroupCode: "task",
		Routes: []RouteDef{
			GET("list", "任务定义列表", AuthModeAuth, deps.TaskCenter.TaskList),
			POST("trigger", "手动触发任务", AuthModeAuth, deps.TaskCenter.Trigger),
		},
		Children: []RouteGroupDef{
			{
				Prefix: "run",
				Routes: []RouteDef{
					GET("list", "任务执行记录列表", AuthModeAuth, deps.TaskCenter.RunList),
					GET("detail", "任务执行记录详情", AuthModeAuth, deps.TaskCenter.RunDetail),
					POST("retry", "重试失败任务", AuthModeAuth, deps.TaskCenter.Retry),
					POST("cancel", "取消任务", AuthModeAuth, deps.TaskCenter.Cancel),
				},
			},
			{
				Prefix: "cron",
				Routes: []RouteDef{
					GET("state", "定时任务最近状态", AuthModeAuth, deps.TaskCenter.CronStateList),
				},
			},
		},
	}
}
