package routers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/middleware"
)

// 路由构建辅助函数（减少重复代码）
func GET(path, title string, handlers ...gin.HandlerFunc) RouteDef {
	return RouteDef{Method: http.MethodGet, Path: path, Title: title, TitleI18n: builtInTitleI18n(title), Handlers: handlers}
}

func POST(path, title string, handlers ...gin.HandlerFunc) RouteDef {
	return RouteDef{Method: http.MethodPost, Path: path, Title: title, TitleI18n: builtInTitleI18n(title), Handlers: handlers}
}

func PATCH(path, title string, handlers ...gin.HandlerFunc) RouteDef {
	return RouteDef{Method: http.MethodPatch, Path: path, Title: title, TitleI18n: builtInTitleI18n(title), Handlers: handlers}
}

// BuildOptions 显式控制构建期条件路由，避免依赖 Gin 进程级 mode。
type BuildOptions struct {
	Debug bool
}

// AdminRouteTree 返回管理员后台路由声明树。
// deps: 控制器依赖容器，传入 nil 则使用默认实现
func AdminRouteTree(deps *ControllerDeps) RouteGroupDef {
	return AdminRouteTreeWithOptions(deps, currentBuildOptions())
}

// AdminRouteTreeWithOptions 使用显式构建参数返回管理员后台路由声明树。
func AdminRouteTreeWithOptions(deps *ControllerDeps, options BuildOptions) RouteGroupDef {
	deps = normalizeControllerDeps(deps)

	return RouteGroupDef{
		Prefix:   "admin/v1",
		Security: SecurityPublic,
		Children: []RouteGroupDef{
			adminPublicGroup(deps, options),
			adminAuthenticatedGroup(deps),
		},
	}
}

// adminPublicGroup 无需登录的公开路由组。
func adminPublicGroup(deps *ControllerDeps, options BuildOptions) RouteGroupDef {
	routes := []RouteDef{
		GET("file/:uuid", "获取文件", middleware.DatabaseReadyGuard(), deps.Common.GetFile),
	}
	if options.Debug {
		routes = append([]RouteDef{
			GET("demo", "Demo 示例", deps.Demo.HelloWorld).WithDesc("Demo 示例备注"),
		}, routes...)
	}
	return RouteGroupDef{
		GroupCode: "other",
		Routes:    routes,
		Children: []RouteGroupDef{
			{
				GroupCode: "login",
				Routes: []RouteDef{
					POST("login", "登录", middleware.DatabaseReadyGuard(), deps.Login.Login).WithDesc("用户登录接口"),
					GET("login-captcha", "验证码", deps.Login.LoginCaptcha).WithDesc("获取登录验证码"),
					GET("login/options", "登录方式", deps.Login.LoginOptions).WithDesc("获取当前启用的登录方式"),
					POST("login/sms/send-code", "发送短信登录验证码", middleware.DatabaseReadyGuard(), deps.Login.SendSMSCode),
					POST("login/sms", "短信验证码登录", middleware.DatabaseReadyGuard(), deps.Login.SMSLogin),
				},
			},
		},
	}
}

// adminAuthenticatedGroup 需要认证的路由组，再按是否需要接口权限分组。
// 认证凭证可能是 access token、refresh cookie 或 WebSocket ticket，具体中间件下沉到子组。
func adminAuthenticatedGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Children: []RouteGroupDef{
			adminNoPermissionGroup(deps),
			adminPermissionGroup(deps),
		},
	}
}

// adminNoPermissionGroup 需要认证但不校验接口权限的路由组。
func adminNoPermissionGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Children: []RouteGroupDef{
			authSessionCredentialGroup(deps),
			websocketTicketGroup(deps),
			adminAccessTokenGroup(deps),
		},
	}
}

// authSessionCredentialGroup 使用 refresh cookie 完成认证或会话清理，不依赖有效 access token。
func authSessionCredentialGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "auth",
		GroupCode: "auth_public",
		Security:  SecurityRefreshCookie,
		Routes: []RouteDef{
			POST("refresh", "刷新 Token", middleware.DatabaseReadyGuard(), middleware.AuthCookieCSRFGuard(), deps.Login.Refresh).WithDesc("通过 HttpOnly refresh_token Cookie 刷新访问令牌"),
			POST("logout", "退出登录", middleware.DatabaseReadyGuard(), middleware.AuthCookieCSRFGuard(), deps.Login.Logout).WithDesc("撤销可用会话凭证并清除 HttpOnly refresh_token Cookie"),
		},
	}
}

// websocketTicketGroup 由 Handler 消费一次性 ticket 并反查登录会话。
func websocketTicketGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "common",
		GroupCode: "common_ws",
		Security:  SecurityWebSocketTicket,
		Routes: []RouteDef{
			GET("ws", "通用实时连接 WebSocket", deps.Message.WS).WithDesc("通过一次性 WebSocket ticket 建立认证连接"),
		},
	}
}

// adminAccessTokenGroup 使用统一 access-token 登录中间件，不校验接口权限。
func adminAccessTokenGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Security: SecurityAccessToken,
		Children: []RouteGroupDef{
			commonGroup(deps),
			authLoginGroup(deps),
			adminProfileGroup(deps),
			notificationLoginGroup(deps),
			personalTodoGroup(deps),
			personalDashboardGroup(deps),
			personalRequestLogGroup(deps),
			personalExportGroup(deps),
		},
	}
}

func personalExportGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "system/export",
		GroupCode: "export_personal",
		Routes: []RouteDef{
			GET("list", "我的导出列表", deps.Export.List),
			GET("summary", "我的导出摘要", deps.Export.Summary),
			GET("detail", "我的导出详情", deps.Export.Detail),
		},
	}
}

// personalTodoGroup 只允许登录用户管理分配给自己的待办。
func personalTodoGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "todo",
		GroupCode: "todo_personal",
		Routes: []RouteDef{
			GET("list", "我的待办列表", deps.Todo.List),
			GET("summary", "我的待办摘要", deps.Todo.Summary),
			GET("groups", "我的待办分组", deps.Todo.Groups),
			GET("reminder-preference", "待办提醒偏好", deps.Todo.ReminderPreference),
			POST("create", "创建个人待办", deps.Todo.Create),
			POST("update", "更新个人待办", deps.Todo.Update),
			POST("status", "更新待办状态", deps.Todo.Status),
			POST("delete", "删除个人待办", deps.Todo.Delete),
			POST("group/create", "创建待办分组", deps.Todo.CreateGroup),
			POST("group/rename", "重命名待办分组", deps.Todo.RenameGroup),
			POST("group/delete", "删除待办分组", deps.Todo.DeleteGroup),
			POST("group/move", "调整待办分组", deps.Todo.MoveGroup),
			POST("reminder-preference", "更新待办提醒偏好", deps.Todo.UpdateReminderPreference),
		},
	}
}

// adminPermissionGroup 需要登录且校验接口权限的路由组。
func adminPermissionGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Security: SecurityAccessTokenPermission,
		Children: []RouteGroupDef{
			dashboardGroup(deps),
			authSessionGroup(deps),
			adminUserGroup(deps),
			permissionGroup(deps),
			menuGroup(deps),
			roleGroup(deps),
			deptGroup(deps),
			productGroup(deps),
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
			POST("upload", "上传文件", deps.Common.Upload),
			POST("ws-ticket", "生成 WebSocket 临时票据", deps.Message.GetWsTicket),
		},
	}
}

func dashboardGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "dashboard",
		GroupCode: "dashboard",
		Routes: []RouteDef{
			GET("overview", "仪表盘概览", deps.Dashboard.Overview),
			GET("statistics", "仪表盘统计", deps.Dashboard.Statistics).WithDesc("默认最近7天；可通过 start_date 和 end_date 查询最多90天的自然日统计"),
			GET("monitor", "服务器监控", deps.Dashboard.Monitor),
		},
	}
}

// personalDashboardGroup 只返回当前登录用户自己的工作台数据。
func personalDashboardGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "dashboard",
		GroupCode: "dashboard_personal",
		Routes: []RouteDef{
			GET("personal", "个人工作台", deps.Dashboard.Personal),
		},
	}
}

// personalRequestLogGroup 只允许当前登录用户查询自己的请求日志。
func personalRequestLogGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "log",
		GroupCode: "request_log_self",
		Children: []RouteGroupDef{
			{
				Prefix: "request",
				Routes: []RouteDef{
					GET("self-list", "个人请求日志列表", deps.RequestLog.SelfList),
					GET("self-detail", "个人请求日志详情", deps.RequestLog.SelfDetail),
				},
			},
		},
	}
}

// authLoginGroup 只要求登录的认证状态接口组。
func authLoginGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "auth",
		GroupCode: "auth",
		Routes: []RouteDef{
			GET("check-token", "检查 Token", deps.Login.CheckToken).WithDesc("验证 Token 有效性"),
		},
	}
}

// authSessionGroup 需要接口权限的在线会话管理组。
func authSessionGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "auth",
		GroupCode: "auth",
		Children: []RouteGroupDef{
			{
				Prefix:    "session",
				GroupCode: "session",
				Routes: []RouteDef{
					GET("list", "在线会话列表", deps.Session.List),
					POST("revoke", "撤销在线会话", deps.Session.Revoke),
				},
			},
		},
	}
}

// adminProfileGroup 只要求登录的个人资料接口组。
func adminProfileGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "admin-user",
		GroupCode: "adminUser",
		Routes: []RouteDef{
			GET("get", "获取当前用户信息", deps.AdminUser.GetUserInfo),
			GET("user-menu-info", "获取用户权限信息", deps.AdminUser.GetUserMenuInfo),
			POST("update-profile", "更新个人资料", deps.AdminUser.UpdateProfile),
			GET("profile-security/options", "个人资料安全选项", deps.AdminUser.ProfileSecurityOptions),
			POST("profile-security/send-code", "发送个人资料验证码", deps.AdminUser.SendProfileCode),
			POST("change-phone", "换绑手机号", deps.AdminUser.ChangePhone),
			POST("avatar/upload", "上传头像", deps.AdminUser.UploadAvatar),
		},
	}
}

// adminUserGroup 需要接口权限的管理员用户管理组。
func adminUserGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "admin-user",
		GroupCode: "adminUser",
		Routes: []RouteDef{
			GET("list", "管理员列表", deps.AdminUser.List),
			GET("options", "管理员选项", deps.AdminUser.Options).WithDesc("按账号/昵称搜索管理员，供下拉选择器使用"),
			GET("detail", "管理员详情", deps.AdminUser.Detail),
			GET("get-full-phone", "获取完整手机号", deps.AdminUser.GetFullPhone).WithDesc("脱敏前完整手机号"),
			GET("get-full-email", "获取完整邮箱", deps.AdminUser.GetFullEmail).WithDesc("脱敏前完整邮箱"),
			POST("create", "新增管理员", deps.AdminUser.Create),
			POST("update", "更新管理员", deps.AdminUser.Update),
			POST("delete", "删除管理员", deps.AdminUser.Delete),
			POST("bind-role", "绑定角色", deps.AdminUser.BindRole),
		},
	}
}

// permissionGroup 接口权限管理组
func permissionGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "permission",
		GroupCode: "permission",
		Routes: []RouteDef{
			POST("update", "更新接口", deps.Api.Update),
			GET("list", "接口列表", deps.Api.List),
			GET("options", "接口选项", deps.Api.Options).WithDesc("按名称/路由搜索接口权限，供菜单编辑时下拉选择器使用"),
		},
	}
}

// menuGroup 菜单管理组
func menuGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "menu",
		GroupCode: "menu",
		Routes: []RouteDef{
			GET("list", "菜单列表", deps.Menu.List),
			POST("delete", "删除菜单", deps.Menu.Delete),
			POST("create", "新增菜单", deps.Menu.Create),
			POST("update", "更新菜单", deps.Menu.Update),
			POST("update-all-menu-permissions", "刷新菜单权限缓存", deps.Menu.UpdateAllMenuPermissions),
			GET("detail", "菜单详情", deps.Menu.Detail),
		},
	}
}

// roleGroup 角色管理组
func roleGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "role",
		GroupCode: "role",
		Routes: []RouteDef{
			GET("list", "角色列表", deps.Role.List),
			GET("options", "角色选项", deps.Role.Options).WithDesc("按名称/编码搜索角色，供下拉选择器使用"),
			POST("create", "新增角色", deps.Role.Create),
			POST("update", "更新角色", deps.Role.Update),
			GET("detail", "角色详情", deps.Role.Detail),
			POST("delete", "删除角色", deps.Role.Delete),
		},
	}
}

// deptGroup 部门管理组
func deptGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "department",
		GroupCode: "department",
		Routes: []RouteDef{
			GET("list", "部门列表", deps.Dept.List),
			POST("create", "新增部门", deps.Dept.Create),
			POST("update", "更新部门", deps.Dept.Update),
			GET("detail", "部门详情", deps.Dept.Detail),
			POST("delete", "删除部门", deps.Dept.Delete),
			POST("bind-role", "部门绑定角色", deps.Dept.BindRole),
		},
	}
}

// productGroup 产品管理组（示例：演示数据权限接入）
func productGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "product",
		GroupCode: "product",
		Routes: []RouteDef{
			GET("list", "产品列表", deps.Product.List),
			POST("create", "新增产品", deps.Product.Create),
			POST("update", "更新产品", deps.Product.Update),
			GET("detail", "产品详情", deps.Product.Detail),
			POST("delete", "删除产品", deps.Product.Delete),
		},
	}
}

// notificationLoginGroup 只要求登录的个人消息接口组。
func notificationLoginGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "system",
		GroupCode: "system",
		Children: []RouteGroupDef{
			{
				Prefix:    "notification",
				GroupCode: "notification",
				Routes: []RouteDef{
					GET("list", "系统消息列表", deps.Message.List),
					GET("unread-count", "系统消息未读数", deps.Message.UnreadCount),
					POST("read", "标记消息已读", deps.Message.Read),
					POST("delete", "删除个人消息", deps.Message.Delete),
					POST("read-all", "全部标记已读", deps.Message.ReadAll),
				},
			},
		},
	}
}

// systemGroup 需要接口权限的系统管理组。
func systemGroup(deps *ControllerDeps) RouteGroupDef {
	return RouteGroupDef{
		Prefix:    "system",
		GroupCode: "system",
		Children: []RouteGroupDef{
			{
				Prefix:    "config",
				GroupCode: "sysConfig",
				Routes: []RouteDef{
					GET("list", "系统参数列表", deps.SysConfig.List),
					GET("detail", "系统参数详情", deps.SysConfig.Detail),
					GET("value", "获取系统参数值", deps.SysConfig.Value),
					POST("create", "新增系统参数", deps.SysConfig.Create),
					POST("update", "更新系统参数", deps.SysConfig.Update),
					POST("delete", "删除系统参数", deps.SysConfig.Delete),
					POST("refresh", "刷新系统参数缓存", deps.SysConfig.Refresh),
				},
			},
			{
				Prefix:    "dict",
				GroupCode: "sysDict",
				Routes: []RouteDef{
					GET("type/list", "字典类型列表", deps.SysDict.TypeList),
					GET("type/detail", "字典类型详情", deps.SysDict.TypeDetail),
					POST("type/create", "新增字典类型", deps.SysDict.TypeCreate),
					POST("type/update", "更新字典类型", deps.SysDict.TypeUpdate),
					POST("type/delete", "删除字典类型", deps.SysDict.TypeDelete),
					GET("item/list", "字典项列表", deps.SysDict.ItemList),
					POST("item/create", "新增字典项", deps.SysDict.ItemCreate),
					POST("item/update", "更新字典项", deps.SysDict.ItemUpdate),
					POST("item/delete", "删除字典项", deps.SysDict.ItemDelete),
					GET("options", "字典选项", deps.SysDict.Options),
				},
			},
			{
				Prefix:    "file",
				GroupCode: "file",
				Routes: []RouteDef{
					GET("list", "文件资源列表", deps.File.List),
					GET("detail", "文件资源详情", deps.File.Detail),
					GET("folder/tree", "文件目录树", deps.File.FolderTree),
					GET("folder/stats", "目录统计信息", deps.File.FolderStats),
					POST("folder/create", "创建文件目录", deps.File.FolderCreate),
					POST("folder/update", "更新文件目录", deps.File.FolderUpdate),
					POST("folder/delete", "删除文件目录", deps.File.FolderDelete),
					POST("folder/move", "移动文件目录", deps.File.FolderMove),
					POST("update", "更新文件资源", deps.File.Update),
					POST("move", "移动文件资源", deps.File.Move),
					GET("upload/options", "文件上传能力", deps.File.UploadOptions),
					POST("upload/local", "本地上传文件资源", deps.File.UploadLocal),
					POST("upload/credential", "获取文件直传凭证", deps.File.UploadCredential),
					POST("upload/credential/batch", "批量获取文件直传凭证", deps.File.UploadCredentialBatch),
					POST("upload/complete", "完成文件直传登记", deps.File.UploadComplete),
					POST("upload/complete/batch", "批量完成文件直传登记", deps.File.UploadCompleteBatch),
					POST("upload/multipart/init", "分片上传初始化", deps.File.MultipartInit),
					POST("upload/multipart/complete", "分片上传完成", deps.File.MultipartComplete),
					POST("upload/multipart/abort", "分片上传中止", deps.File.MultipartAbort),
					POST("delete", "删除文件资源", deps.File.Delete),
					POST("batch-delete", "批量删除文件资源", deps.File.BatchDelete),
					GET("trash/list", "文件回收站列表", deps.File.TrashList),
					POST("trash/restore", "恢复文件资源", deps.File.Restore),
					POST("trash/batch-restore", "批量恢复文件资源", deps.File.BatchRestore),
					POST("trash/destroy", "硬删除文件资源", deps.File.Destroy),
					POST("trash/batch-destroy", "批量硬删除文件资源", deps.File.BatchDestroy),

					GET("references", "文件引用列表", deps.File.References),
					POST("export", "导出文件资源", deps.File.Export),
				},
			},
			{
				Prefix:    "notification",
				GroupCode: "notification",
				Routes: []RouteDef{
					POST("send", "发送系统通知", deps.Message.Send),
				},
			},
			{
				Prefix:    "storage",
				GroupCode: "storage",
				Routes: []RouteDef{
					GET("config", "存储配置", deps.Storage.Config),
					GET("secret", "查看存储敏感信息", deps.Storage.Secret),
					POST("config", "保存存储配置", deps.Storage.Save),
					POST("test", "测试存储配置", deps.Storage.Test),
				},
			},
			{
				Prefix:    "sms",
				GroupCode: "sms",
				Routes: []RouteDef{
					GET("config", "短信配置", deps.SMS.Config),
					POST("config", "保存短信配置", deps.SMS.Save),
					POST("test", "测试短信发送", deps.SMS.Test).WithDesc("使用当前未保存的短信配置草稿发送测试短信，草稿不会入库"),
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
					GET("list", "请求日志列表", deps.RequestLog.List),
					GET("detail", "请求日志详情", deps.RequestLog.Detail),
					POST("export", "导出请求日志", deps.RequestLog.Export),
					GET("mask-config", "获取数据脱敏配置", deps.RequestLog.MaskConfig),
					POST("mask-config", "更新数据脱敏配置", deps.RequestLog.UpdateMaskConfig),
				},
			},
			{
				Prefix: "login",
				Routes: []RouteDef{
					GET("list", "登录日志列表", deps.LoginLog.List),
					GET("detail", "登录日志详情", deps.LoginLog.Detail),
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
			GET("list", "任务定义列表", deps.TaskCenter.TaskList),
			POST("trigger", "手动触发任务", deps.TaskCenter.Trigger),
			POST("record-policy", "更新任务记录策略", deps.TaskCenter.RecordPolicy),
			POST("operation-config", "更新任务运营配置", deps.TaskCenter.OperationConfig),
		},
		Children: []RouteGroupDef{
			{
				Prefix: "run",
				Routes: []RouteDef{
					GET("list", "任务执行记录列表", deps.TaskCenter.RunList).WithDesc("time_basis 可选择 created_at 或 finished_at，默认按 created_at 筛选"),
					GET("stats", "任务执行统计", deps.TaskCenter.RunStats).WithDesc("按完成时间小时桶统计，并返回失败类明细下钻的完成时间范围"),
					GET("stats/trend", "任务执行统计趋势", deps.TaskCenter.RunStatsTrend),
					GET("detail", "任务执行记录详情", deps.TaskCenter.RunDetail),
					GET("events", "任务执行事件列表", deps.TaskCenter.RunEvents),
					POST("retry", "重试失败任务", deps.TaskCenter.Retry),
					POST("cancel", "取消任务", deps.TaskCenter.Cancel),
				},
			},
			{
				Prefix: "cron",
				Routes: []RouteDef{
					GET("state", "定时任务最近状态", deps.TaskCenter.CronStateList),
				},
			},
		},
	}
}
