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
			reqNotAuth.get("demo", demo.HelloWorld).setTitle("Demo 示例").setAuth(0)

			loginC := admin_v1.NewLoginController()
			lg := reqNotAuth.group("").setGroupCode("login")
			{
				lg.post("login", loginC.Login).setTitle("登录").setAuth(0)
				lg.get("login-captcha", loginC.LoginCaptcha).setTitle("验证码").setAuth(0)
			}
		}
		/******************************* 无需校验权限 end **************************************/

		/******************************* 需校验权限 start **************************************/
		reqAuth := v1.group("", middleware.AdminAuthHandler())
		{
			// 授权管理
			auth := reqAuth.group("auth").setGroupCode("auth")
			{
				c := admin_v1.NewLoginController()
				auth.post("logout", c.Logout).setTitle("退出登录").setAuth(0)
			}

			// 管理员用户
			adminUser := reqAuth.group("admin-user").setGroupCode("adminUser")
			{
				c := admin_v1.NewAdminUserController()
				adminUser.get("get", c.GetUserInfo).setTitle("获取管理员用户信息").setAuth(0)
				adminUser.get("list", c.List).setTitle("获取管理员用户列表").setAuth(1)
				adminUser.post("edit", c.Edit).setTitle("编辑管理员信息").setAuth(1)
				adminUser.post("delete", c.Delete).setTitle("删除管理员").setAuth(1)
			}

			// api权限管理
			permissions := reqAuth.group("permission").setGroupCode("api")
			{
				c := admin_v1.NewApiController()
				permissions.post("edit", c.Edit).setTitle("编辑权限").setAuth(1)
				permissions.get("list", c.List).setTitle("权限列表").setAuth(1)
			}

			// 菜单管理
			menu := reqAuth.group("menu").setGroupCode("menu")
			{
				c := admin_v1.NewMenuController()
				menu.get("list", c.List).setTitle("菜单列表").setAuth(1)
				menu.post("delete", c.Delete).setTitle("删除菜单").setAuth(1)
				menu.post("edit", c.Edit).setTitle("编辑菜单").setAuth(1)
				menu.get("detail", c.Detail).setTitle("菜单详情").setAuth(1)
			}

			// 角色管理
			role := reqAuth.group("role").setGroupCode("role")
			{
				c := admin_v1.NewRoleController()
				role.get("list", c.List).setTitle("获取角色列表").setAuth(1)
				role.post("edit", c.Edit).setTitle("编辑角色").setAuth(1)
				role.post("delete", c.Delete).setTitle("删除角色").setAuth(1)
			}

			// 用户组管理
			group := reqAuth.group("group").setGroupCode("department")
			{
				c := admin_v1.NewAdminUserController()
				group.get("get", c.GetUserInfo).setTitle("获取用户组信息").setAuth(1)
			}
		}
		/******************************* 需校验权限 end ****************************************/
	}
}
