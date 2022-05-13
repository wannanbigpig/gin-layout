package routers

import (
	"github.com/gin-gonic/gin"
	c "github.com/wannanbigpig/gin-layout/internal/controller"
)

func setApiRoute(r *gin.Engine) {
	// version 1
	v1 := r.Group("/api/v1")
	{
		v1.POST("/login", c.Login)
		//v1.GET("/register", controller.Register)
	}
}
