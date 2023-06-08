package routers

import (
	"github.com/gin-gonic/gin"
	controller "github.com/wannanbigpig/gin-layout/internal/controller"
	admin_v1 "github.com/wannanbigpig/gin-layout/internal/controller/admin_v1"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
)

func SetAdminApiRoute(r *gin.Engine) {
	// version 1
	v1 := r.Group("api/v1")
	{
		demo := controller.NewDemoController()
		v1.GET("hello-world", demo.HelloWorld)
		// 无需校验权限
		loginC := admin_v1.NewLoginController()
		v1.POST("admin/login", loginC.Login)

		// 需要校验权限
		reqAuth := v1.Group("", middleware.AdminAuthHandler())
		{
			// 管理员用户
			adminUser := reqAuth.Group("admin-user")
			{
				adminUserC := admin_v1.NewAdminUserController()
				adminUser.GET("get", adminUserC.GetUserInfo)
			}

		}
	}
}
