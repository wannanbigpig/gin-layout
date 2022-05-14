package routers

import (
	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/pkg/error_code"
	response2 "github.com/wannanbigpig/gin-layout/pkg/response"
	"io/ioutil"
	"net/http"
)

func SetRouters() *gin.Engine {
	var r *gin.Engine

	if config.Config.AppEnv == "prod" {
		// 生产模式
		r = ReleaseRouter()
	} else {
		// 调试模式
		r = gin.New()
	}

	r.Use(middleware.RequestCostHandler(), gin.Logger(), gin.Recovery(), middleware.CorsHandler())

	// ping
	r.GET("/ping", func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusOK, gin.H{
			"message": "pong!",
		})
	})

	// 设置 API 路由
	setApiRoute(r)

	r.NoRoute(func(c *gin.Context) {
		response2.NewResponse().SetHttpCode(http.StatusNotFound).FailCode(c, error_code.ServerError, "资源不存在")
	})

	return r
}

// ReleaseRouter 生产模式使用官方建议设置为 release 模式
func ReleaseRouter() *gin.Engine {
	// 切换到生产模式
	gin.SetMode(gin.ReleaseMode)
	// 禁用 gin 输出接口访问日志
	gin.DefaultWriter = ioutil.Discard

	engine := gin.New()
	// 载入gin的中间件
	engine.Use(gin.Logger(), middleware.CustomRecovery())
	return engine
}
