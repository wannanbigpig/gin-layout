package routers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	response2 "github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

// SetRouters 创建 Gin 引擎并注册全部应用路由。
func SetRouters() *gin.Engine {
	engine := createEngine()
	RegisterRoutes(engine, AppRouteTree())

	// 统一处理 404
	engine.NoRoute(func(c *gin.Context) {
		response2.Resp().SetHttpCode(http.StatusNotFound).FailCode(c, errors.NotFound)
	})

	return engine
}

// createEngine 创建 gin 引擎并设置相关中间件
func createEngine() *gin.Engine {
	var engine *gin.Engine

	if config.Config.Debug {
		// 开发调试模式
		engine = gin.New()
		engine.Use(
			middleware.CorsHandler(),
			middleware.RequestCostHandler(), // 请求耗时统计
			middleware.ParseTokenHandler(),  // 全局token解析（所有路由都走）
			gin.Logger(),
			middleware.CustomRecovery(),
			middleware.CustomLogger(),
		)

	} else {
		// 生产模式
		engine = ReleaseRouter()
		engine.Use(
			middleware.CorsHandler(),
			middleware.RequestCostHandler(), // 请求耗时统计
			middleware.ParseTokenHandler(),  // 全局token解析（所有路由都走）
			middleware.CustomRecovery(),
			middleware.CustomLogger(),
		)
	}
	// 配置受信任代理，决定是否信任 X-Forwarded-For / X-Real-IP 等代理头。
	if err := engine.SetTrustedProxies(config.Config.TrustedProxies); err != nil {
		panic(err)
	}

	return engine
}

// ReleaseRouter 生产模式使用官方建议设置为 release 模式
func ReleaseRouter() *gin.Engine {
	// 切换到生产模式
	gin.SetMode(gin.ReleaseMode)
	// 禁用 gin 输出接口访问日志
	gin.DefaultWriter = io.Discard

	engine := gin.New()

	return engine
}

// AppRouteTree 返回应用完整路由树。
func AppRouteTree() RouteGroupDef {
	return RouteGroupDef{
		Routes: []RouteDef{
			{
				Method: http.MethodGet,
				Path:   "ping",
				Title:  "ping",
				Desc:   "服务心跳检测接口",
				Auth:   0,
				Handlers: []gin.HandlerFunc{func(c *gin.Context) {
					c.String(http.StatusOK, "pong")
				}},
			},
		},
		Children: []RouteGroupDef{AdminRouteTree()},
	}
}
