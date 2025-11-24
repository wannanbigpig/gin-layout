package routers

import (
	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/controller/admin_v1"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
)

// SetAdminApiRoute 设置管理员API路由并保存权限信息
func SetAdminApiRoute(r *RegisterRouter) {
	// version 1
	v1 := r.group("admin/v1/")
	{
		/******************************* 无需校验权限 start ************************************/
		reqNotAuth := v1.group("").setGroupCode("other")
		{
			demo := controller.NewDemoController()
			reqNotAuth.get("demo", demo.HelloWorld).setTitle("Demo 示例").setDesc("Demo 示例备注").setAuth(0)

			loginC := admin_v1.NewLoginController()
			lg := reqNotAuth.group("").setGroupCode("login")
			{
				lg.post("login", loginC.Login).setTitle("登录").setAuth(0)
				lg.get("login-captcha", loginC.LoginCaptcha).setTitle("验证码").setAuth(0)
			}
			
			// 文件访问（公开文件无需认证，私有文件会在控制器中检查）
			// 使用UUID查询（32位字符串），比64位hash更短，适合URL
			commonC := admin_v1.NewCommonController()
			reqNotAuth.get("file/:uuid", commonC.GetFile).setTitle("获取文件").setAuth(0)
		}
		/******************************* 无需校验权限 end **************************************/

		/******************************* 需校验权限 start **************************************/
		reqAuth := v1.group("", middleware.AdminAuthHandler())
		{
			common := reqAuth.group("common").setGroupCode("common")
			{
				c := admin_v1.NewCommonController()
				common.post("upload", c.Upload).setTitle("上传文件").setAuth(0)
			}

			// 授权管理
			auth := reqAuth.group("auth").setGroupCode("auth")
			{
				c := admin_v1.NewLoginController()
				auth.post("logout", c.Logout).setTitle("退出登录").setAuth(0)
				auth.get("check-token", c.CheckToken).setTitle("检查Token是否有效").setAuth(0)
			}

			// 管理员用户
			adminUser := reqAuth.group("admin-user").setGroupCode("adminUser")
			{
				c := admin_v1.NewAdminUserController()
				adminUser.get("get", c.GetUserInfo).setTitle("获取当前登录用户基本信息").setAuth(0)
				adminUser.get("user-menu-info", c.GetUserMenuInfo).setTitle("获取当前登录用户权限信息").setAuth(0)
				adminUser.post("update-profile", c.UpdateProfile).setTitle("更新个人资料").setAuth(0)
				adminUser.get("list", c.List).setTitle("获取管理员用户列表").setAuth(1)
				adminUser.get("detail", c.Detail).setTitle("获取管理员详情").setAuth(1)
				adminUser.get("get-full-phone", c.GetFullPhone).setTitle("获取管理员完整手机号").setAuth(1)
				adminUser.get("get-full-email", c.GetFullEmail).setTitle("获取管理员完整邮箱").setAuth(1)
				adminUser.post("create", c.Create).setTitle("新增管理员").setAuth(1)
				adminUser.post("update", c.Update).setTitle("更新管理员").setAuth(1)
				adminUser.post("delete", c.Delete).setTitle("删除管理员").setAuth(1)
				adminUser.post("bind-role", c.BindRole).setTitle("管理员绑定角色").setAuth(1)
			}

			// api权限管理
			permissions := reqAuth.group("permission").setGroupCode("api")
			{
				c := admin_v1.NewApiController()
				// permissions.post("create", c.Create).setTitle("新增接口").setAuth(1)
				permissions.post("update", c.Update).setTitle("更新接口").setAuth(1)
				permissions.get("list", c.List).setTitle("接口列表").setAuth(1)
			}

			// 菜单管理
			menu := reqAuth.group("menu").setGroupCode("menu")
			{
				c := admin_v1.NewMenuController()
				menu.get("list", c.List).setTitle("菜单列表").setAuth(1)
				menu.post("delete", c.Delete).setTitle("删除菜单").setAuth(1)
				menu.post("create", c.Create).setTitle("新增菜单").setAuth(1)
				menu.post("update", c.Update).setTitle("更新菜单").setAuth(1)
				menu.post("update-all-menu-permissions", c.UpdateAllMenuPermissions).setTitle("更新全部菜单权限关系缓存").setAuth(1)
				menu.get("detail", c.Detail).setTitle("菜单详情").setAuth(1)
			}

			// 角色管理
			role := reqAuth.group("role").setGroupCode("role")
			{
				c := admin_v1.NewRoleController()
				role.get("list", c.List).setTitle("获取角色列表").setAuth(1)
				role.post("create", c.Create).setTitle("新增角色").setAuth(1)
				role.post("update", c.Update).setTitle("更新角色").setAuth(1)
				role.get("detail", c.Detail).setTitle("角色详情").setAuth(1)
				role.post("delete", c.Delete).setTitle("删除角色").setAuth(1)
			}

			// 部门管理
			dept := reqAuth.group("department").setGroupCode("department")
			{
				c := admin_v1.NewDeptController()
				dept.get("list", c.List).setTitle("获取部门列表").setAuth(1)
				dept.post("create", c.Create).setTitle("新增部门").setAuth(1)
				dept.post("update", c.Update).setTitle("更新部门").setAuth(1)
				dept.get("detail", c.Detail).setTitle("部门详情").setAuth(1)
				dept.post("delete", c.Delete).setTitle("删除部门").setAuth(1)
				dept.post("bind-role", c.BindRole).setTitle("部门绑定角色").setAuth(1)
			}

			// 日志管理
			log := reqAuth.group("log").setGroupCode("log")
			{
				// 请求日志
				requestLog := log.group("request")
				{
					requestLogC := admin_v1.NewRequestLogController()
					requestLog.get("list", requestLogC.List).setTitle("获取请求日志列表").setAuth(1)
					requestLog.get("detail", requestLogC.Detail).setTitle("请求日志详情").setAuth(1)
				}
				// 登录日志
				loginLog := log.group("login")
				{
					loginLogC := admin_v1.NewAdminLoginLogController()
					loginLog.get("list", loginLogC.List).setTitle("获取登录日志列表").setAuth(1)
					loginLog.get("detail", loginLogC.Detail).setTitle("登录日志详情").setAuth(1)
				}
			}
		}
		/******************************* 需校验权限 end ****************************************/
	}
}
