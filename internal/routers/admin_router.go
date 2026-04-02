package routers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/controller/admin_v1"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
)

// AdminRouteTree 返回管理员后台路由声明树。
func AdminRouteTree() RouteGroupDef {
	demo := controller.NewDemoController()
	loginC := admin_v1.NewLoginController()
	commonC := admin_v1.NewCommonController()
	adminUserC := admin_v1.NewAdminUserController()
	apiC := admin_v1.NewApiController()
	menuC := admin_v1.NewMenuController()
	roleC := admin_v1.NewRoleController()
	deptC := admin_v1.NewDeptController()
	requestLogC := admin_v1.NewRequestLogController()
	loginLogC := admin_v1.NewAdminLoginLogController()

	return RouteGroupDef{
		Prefix: "admin/v1",
		Children: []RouteGroupDef{
			{
				GroupCode: "other",
				Routes: []RouteDef{
					{Method: http.MethodGet, Path: "demo", Title: "Demo 示例", Desc: "Demo 示例备注", Auth: 0, Handlers: []gin.HandlerFunc{demo.HelloWorld}},
					{Method: http.MethodGet, Path: "file/:uuid", Title: "获取文件", Auth: 0, Handlers: []gin.HandlerFunc{commonC.GetFile}},
				},
				Children: []RouteGroupDef{
					{
						GroupCode: "login",
						Routes: []RouteDef{
							{Method: http.MethodPost, Path: "login", Title: "登录", Auth: 0, Handlers: []gin.HandlerFunc{loginC.Login}},
							{Method: http.MethodGet, Path: "login-captcha", Title: "验证码", Auth: 0, Handlers: []gin.HandlerFunc{loginC.LoginCaptcha}},
						},
					},
				},
			},
			{
				Middleware: []gin.HandlerFunc{middleware.AdminAuthHandler()},
				Children: []RouteGroupDef{
					{
						Prefix:    "common",
						GroupCode: "common",
						Routes: []RouteDef{
							{Method: http.MethodPost, Path: "upload", Title: "上传文件", Auth: 0, Handlers: []gin.HandlerFunc{commonC.Upload}},
						},
					},
					{
						Prefix:    "auth",
						GroupCode: "auth",
						Routes: []RouteDef{
							{Method: http.MethodPost, Path: "logout", Title: "退出登录", Auth: 0, Handlers: []gin.HandlerFunc{loginC.Logout}},
							{Method: http.MethodGet, Path: "check-token", Title: "检查Token是否有效", Auth: 0, Handlers: []gin.HandlerFunc{loginC.CheckToken}},
						},
					},
					{
						Prefix:    "admin-user",
						GroupCode: "adminUser",
						Routes: []RouteDef{
							{Method: http.MethodGet, Path: "get", Title: "获取当前登录用户基本信息", Auth: 0, Handlers: []gin.HandlerFunc{adminUserC.GetUserInfo}},
							{Method: http.MethodGet, Path: "user-menu-info", Title: "获取当前登录用户权限信息", Auth: 0, Handlers: []gin.HandlerFunc{adminUserC.GetUserMenuInfo}},
							{Method: http.MethodPost, Path: "update-profile", Title: "更新个人资料", Auth: 0, Handlers: []gin.HandlerFunc{adminUserC.UpdateProfile}},
							{Method: http.MethodGet, Path: "list", Title: "获取管理员用户列表", Auth: 1, Handlers: []gin.HandlerFunc{adminUserC.List}},
							{Method: http.MethodGet, Path: "detail", Title: "获取管理员详情", Auth: 1, Handlers: []gin.HandlerFunc{adminUserC.Detail}},
							{Method: http.MethodGet, Path: "get-full-phone", Title: "获取管理员完整手机号", Auth: 1, Handlers: []gin.HandlerFunc{adminUserC.GetFullPhone}},
							{Method: http.MethodGet, Path: "get-full-email", Title: "获取管理员完整邮箱", Auth: 1, Handlers: []gin.HandlerFunc{adminUserC.GetFullEmail}},
							{Method: http.MethodPost, Path: "create", Title: "新增管理员", Auth: 1, Handlers: []gin.HandlerFunc{adminUserC.Create}},
							{Method: http.MethodPost, Path: "update", Title: "更新管理员", Auth: 1, Handlers: []gin.HandlerFunc{adminUserC.Update}},
							{Method: http.MethodPost, Path: "delete", Title: "删除管理员", Auth: 1, Handlers: []gin.HandlerFunc{adminUserC.Delete}},
							{Method: http.MethodPost, Path: "bind-role", Title: "管理员绑定角色", Auth: 1, Handlers: []gin.HandlerFunc{adminUserC.BindRole}},
						},
					},
					{
						Prefix:    "permission",
						GroupCode: "api",
						Routes: []RouteDef{
							{Method: http.MethodPost, Path: "update", Title: "更新接口", Auth: 1, Handlers: []gin.HandlerFunc{apiC.Update}},
							{Method: http.MethodGet, Path: "list", Title: "接口列表", Auth: 1, Handlers: []gin.HandlerFunc{apiC.List}},
						},
					},
					{
						Prefix:    "menu",
						GroupCode: "menu",
						Routes: []RouteDef{
							{Method: http.MethodGet, Path: "list", Title: "菜单列表", Auth: 1, Handlers: []gin.HandlerFunc{menuC.List}},
							{Method: http.MethodPost, Path: "delete", Title: "删除菜单", Auth: 1, Handlers: []gin.HandlerFunc{menuC.Delete}},
							{Method: http.MethodPost, Path: "create", Title: "新增菜单", Auth: 1, Handlers: []gin.HandlerFunc{menuC.Create}},
							{Method: http.MethodPost, Path: "update", Title: "更新菜单", Auth: 1, Handlers: []gin.HandlerFunc{menuC.Update}},
							{Method: http.MethodPost, Path: "update-all-menu-permissions", Title: "更新全部菜单权限关系缓存", Auth: 1, Handlers: []gin.HandlerFunc{menuC.UpdateAllMenuPermissions}},
							{Method: http.MethodGet, Path: "detail", Title: "菜单详情", Auth: 1, Handlers: []gin.HandlerFunc{menuC.Detail}},
						},
					},
					{
						Prefix:    "role",
						GroupCode: "role",
						Routes: []RouteDef{
							{Method: http.MethodGet, Path: "list", Title: "获取角色列表", Auth: 1, Handlers: []gin.HandlerFunc{roleC.List}},
							{Method: http.MethodPost, Path: "create", Title: "新增角色", Auth: 1, Handlers: []gin.HandlerFunc{roleC.Create}},
							{Method: http.MethodPost, Path: "update", Title: "更新角色", Auth: 1, Handlers: []gin.HandlerFunc{roleC.Update}},
							{Method: http.MethodGet, Path: "detail", Title: "角色详情", Auth: 1, Handlers: []gin.HandlerFunc{roleC.Detail}},
							{Method: http.MethodPost, Path: "delete", Title: "删除角色", Auth: 1, Handlers: []gin.HandlerFunc{roleC.Delete}},
						},
					},
					{
						Prefix:    "department",
						GroupCode: "department",
						Routes: []RouteDef{
							{Method: http.MethodGet, Path: "list", Title: "获取部门列表", Auth: 1, Handlers: []gin.HandlerFunc{deptC.List}},
							{Method: http.MethodPost, Path: "create", Title: "新增部门", Auth: 1, Handlers: []gin.HandlerFunc{deptC.Create}},
							{Method: http.MethodPost, Path: "update", Title: "更新部门", Auth: 1, Handlers: []gin.HandlerFunc{deptC.Update}},
							{Method: http.MethodGet, Path: "detail", Title: "部门详情", Auth: 1, Handlers: []gin.HandlerFunc{deptC.Detail}},
							{Method: http.MethodPost, Path: "delete", Title: "删除部门", Auth: 1, Handlers: []gin.HandlerFunc{deptC.Delete}},
							{Method: http.MethodPost, Path: "bind-role", Title: "部门绑定角色", Auth: 1, Handlers: []gin.HandlerFunc{deptC.BindRole}},
						},
					},
					{
						Prefix:    "log",
						GroupCode: "log",
						Children: []RouteGroupDef{
							{
								Prefix: "request",
								Routes: []RouteDef{
									{Method: http.MethodGet, Path: "list", Title: "获取请求日志列表", Auth: 1, Handlers: []gin.HandlerFunc{requestLogC.List}},
									{Method: http.MethodGet, Path: "detail", Title: "请求日志详情", Auth: 1, Handlers: []gin.HandlerFunc{requestLogC.Detail}},
								},
							},
							{
								Prefix: "login",
								Routes: []RouteDef{
									{Method: http.MethodGet, Path: "list", Title: "获取登录日志列表", Auth: 1, Handlers: []gin.HandlerFunc{loginLogC.List}},
									{Method: http.MethodGet, Path: "detail", Title: "登录日志详情", Auth: 1, Handlers: []gin.HandlerFunc{loginLogC.Detail}},
								},
							},
						},
					},
				},
			},
		},
	}
}
