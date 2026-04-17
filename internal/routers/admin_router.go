package routers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/middleware"
)

// 路由构建辅助函数（减少重复代码）
func GET(path, title, desc string, auth AuthMode, handlers ...gin.HandlerFunc) RouteDef {
	return RouteDef{Method: http.MethodGet, Path: path, Title: title, Desc: desc, Auth: auth, Handlers: handlers}
}

func POST(path, title, desc string, auth AuthMode, handlers ...gin.HandlerFunc) RouteDef {
	return RouteDef{Method: http.MethodPost, Path: path, Title: title, Desc: desc, Auth: auth, Handlers: handlers}
}

// AdminRouteTree 返回管理员后台路由声明树。
// deps: 控制器依赖容器，传入 nil 则使用默认实现
func AdminRouteTree(deps *ControllerDeps) RouteGroupDef {
	if deps == nil {
		deps = DefaultControllerDeps()
	}

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
			GET("demo", "Demo 示例", "Demo 示例备注", AuthModeNone, deps.Demo.HelloWorld),
			GET("file/:uuid", "获取文件", "", AuthModeNone, middleware.DatabaseReadyGuard(), deps.Common.GetFile),
		},
		Children: []RouteGroupDef{
			{
				GroupCode: "login",
				Routes: []RouteDef{
					POST("login", "登录", "用户登录接口", AuthModeNone, middleware.DatabaseReadyGuard(), deps.Login.Login),
					GET("login-captcha", "验证码", "获取登录验证码", AuthModeNone, deps.Login.LoginCaptcha),
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
			logGroup(deps),
		},
	}
}

// commonGroup 通用接口组
func commonGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "common",
		GroupCode: "common",
		Routes: []RouteDef{
			POST("upload", "上传文件", "", AuthModeLogin, deps.Common.Upload),
		},
	}
}

// authGroup 认证相关接口组
func authGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "auth",
		GroupCode: "auth",
		Routes: []RouteDef{
			POST("logout", "退出登录", "", AuthModeLogin, deps.Login.Logout),
			GET("check-token", "检查 Token", "验证 Token 有效性", AuthModeLogin, deps.Login.CheckToken),
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
			GET("get", "获取当前用户信息", "", AuthModeLogin, deps.AdminUser.GetUserInfo),
			GET("user-menu-info", "获取用户权限信息", "", AuthModeLogin, deps.AdminUser.GetUserMenuInfo),
			POST("update-profile", "更新个人资料", "", AuthModeLogin, deps.AdminUser.UpdateProfile),

			// 用户管理（AuthModeAuthz：需要权限）
			GET("list", "管理员列表", "", AuthModeAuthz, deps.AdminUser.List),
			GET("detail", "管理员详情", "", AuthModeAuthz, deps.AdminUser.Detail),
			GET("get-full-phone", "获取完整手机号", "脱敏前完整手机号", AuthModeAuthz, deps.AdminUser.GetFullPhone),
			GET("get-full-email", "获取完整邮箱", "脱敏前完整邮箱", AuthModeAuthz, deps.AdminUser.GetFullEmail),
			POST("create", "新增管理员", "", AuthModeAuthz, deps.AdminUser.Create),
			POST("update", "更新管理员", "", AuthModeAuthz, deps.AdminUser.Update),
			POST("delete", "删除管理员", "", AuthModeAuthz, deps.AdminUser.Delete),
			POST("bind-role", "绑定角色", "", AuthModeAuthz, deps.AdminUser.BindRole),
		},
	}
}

// permissionGroup 接口权限管理组
func permissionGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "permission",
		GroupCode: "api",
		Routes: []RouteDef{
			POST("update", "更新接口", "", AuthModeAuthz, deps.Api.Update),
			GET("list", "接口列表", "", AuthModeAuthz, deps.Api.List),
		},
	}
}

// menuGroup 菜单管理组
func menuGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "menu",
		GroupCode: "menu",
		Routes: []RouteDef{
			GET("list", "菜单列表", "", AuthModeAuthz, deps.Menu.List),
			POST("delete", "删除菜单", "", AuthModeAuthz, deps.Menu.Delete),
			POST("create", "新增菜单", "", AuthModeAuthz, deps.Menu.Create),
			POST("update", "更新菜单", "", AuthModeAuthz, deps.Menu.Update),
			POST("update-all-menu-permissions", "刷新菜单权限缓存", "", AuthModeAuthz, deps.Menu.UpdateAllMenuPermissions),
			GET("detail", "菜单详情", "", AuthModeAuthz, deps.Menu.Detail),
		},
	}
}

// roleGroup 角色管理组
func roleGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "role",
		GroupCode: "role",
		Routes: []RouteDef{
			GET("list", "角色列表", "", AuthModeAuthz, deps.Role.List),
			POST("create", "新增角色", "", AuthModeAuthz, deps.Role.Create),
			POST("update", "更新角色", "", AuthModeAuthz, deps.Role.Update),
			GET("detail", "角色详情", "", AuthModeAuthz, deps.Role.Detail),
			POST("delete", "删除角色", "", AuthModeAuthz, deps.Role.Delete),
		},
	}
}

// deptGroup 部门管理组
func deptGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "department",
		GroupCode: "department",
		Routes: []RouteDef{
			GET("list", "部门列表", "", AuthModeAuthz, deps.Dept.List),
			POST("create", "新增部门", "", AuthModeAuthz, deps.Dept.Create),
			POST("update", "更新部门", "", AuthModeAuthz, deps.Dept.Update),
			GET("detail", "部门详情", "", AuthModeAuthz, deps.Dept.Detail),
			POST("delete", "删除部门", "", AuthModeAuthz, deps.Dept.Delete),
			POST("bind-role", "部门绑定角色", "", AuthModeAuthz, deps.Dept.BindRole),
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
					GET("list", "请求日志列表", "", AuthModeAuthz, deps.RequestLog.List),
					GET("detail", "请求日志详情", "", AuthModeAuthz, deps.RequestLog.Detail),
				},
			},
			{
				Prefix: "login",
				Routes: []RouteDef{
					GET("list", "登录日志列表", "", AuthModeAuthz, deps.LoginLog.List),
					GET("detail", "登录日志详情", "", AuthModeAuthz, deps.LoginLog.Detail),
				},
			},
		},
	}
}
