package routers

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	response2 "github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"github.com/wannanbigpig/gin-layout/internal/queue"
)

// SetRouters 创建 Gin 引擎并注册全部应用路由。
func SetRouters() (*gin.Engine, error) {
	return SetRoutersWithTree(AppRouteTree())
}

// SetRoutersWithTree 使用指定路由树创建 Gin 引擎并注册路由。
func SetRoutersWithTree(routeTree RouteGroupDef) (*gin.Engine, error) {
	// 启动时校验路由树
	if err := ValidateRouteTree(routeTree); err != nil {
		return nil, fmt.Errorf("route tree validation failed: %w", err)
	}

	engine, err := createEngine()
	if err != nil {
		return nil, err
	}
	RegisterRoutes(engine, routeTree)

	// 统一处理 404
	engine.NoRoute(func(c *gin.Context) {
		response2.Resp().SetHttpCode(http.StatusNotFound).FailCode(c, errors.NotFound)
	})

	return engine, nil
}

// createEngine 创建 gin 引擎并设置相关中间件
func createEngine() (*gin.Engine, error) {
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
		return nil, fmt.Errorf("set trusted proxies failed: %w", err)
	}

	return engine, nil
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
				Auth:   AuthModeNone,
				Handlers: []gin.HandlerFunc{func(c *gin.Context) {
					c.String(http.StatusOK, "pong")
				}},
			},
			{
				Method: http.MethodGet,
				Path:   "health/readiness",
				Title:  "readiness",
				Desc:   "服务依赖就绪状态",
				Auth:   AuthModeNone,
				Handlers: []gin.HandlerFunc{func(c *gin.Context) {
					status := buildReadinessStatus()
					httpCode := http.StatusOK
					if !status.Ready {
						httpCode = http.StatusServiceUnavailable
					}
					c.JSON(httpCode, status)
				}},
			},
		},
		Children: []RouteGroupDef{AdminRouteTree(nil)},
	}
}

type readinessStatus struct {
	Ready        bool               `json:"ready"`
	Timestamp    string             `json:"timestamp"`
	Dependencies readinessComponent `json:"dependencies"`
}

type readinessComponent struct {
	Mysql dependencyStatus `json:"mysql"`
	Redis dependencyStatus `json:"redis"`
	Queue dependencyStatus `json:"queue"`
}

type dependencyStatus struct {
	Enabled  bool   `json:"enabled"`
	Required bool   `json:"required"`
	Ready    bool   `json:"ready"`
	Message  string `json:"message,omitempty"`
}

func buildReadinessStatus() readinessStatus {
	cfg := config.GetConfig()
	mysqlStatus := buildMySQLReadiness(cfg)
	redisStatus := buildRedisReadiness(cfg)
	queueStatus := buildQueueReadiness(cfg)

	ready := mysqlStatus.Ready
	if redisStatus.Enabled && !redisStatus.Ready {
		ready = false
	}
	if queueStatus.Enabled && !queueStatus.Ready {
		ready = false
	}

	return readinessStatus{
		Ready:     ready,
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Dependencies: readinessComponent{
			Mysql: mysqlStatus,
			Redis: redisStatus,
			Queue: queueStatus,
		},
	}
}

func buildMySQLReadiness(cfg *config.Conf) dependencyStatus {
	if cfg == nil || !cfg.Mysql.Enable {
		return dependencyStatus{
			Enabled:  false,
			Required: true,
			Ready:    false,
			Message:  "mysql is disabled",
		}
	}

	mysqlStatus := data.MysqlRuntimeStatus()
	if mysqlStatus.Ready {
		return dependencyStatus{
			Enabled:  true,
			Required: true,
			Ready:    true,
		}
	}

	message := "mysql connection is unavailable"
	if mysqlStatus.Error != nil {
		message = mysqlStatus.Error.Error()
	}
	return dependencyStatus{
		Enabled:  true,
		Required: true,
		Ready:    false,
		Message:  message,
	}
}

func buildRedisReadiness(cfg *config.Conf) dependencyStatus {
	if cfg == nil || !cfg.Redis.Enable {
		return dependencyStatus{
			Enabled:  false,
			Required: false,
			Ready:    false,
			Message:  "redis is disabled",
		}
	}

	redisStatus := data.RedisRuntimeStatus()
	if redisStatus.Ready {
		return dependencyStatus{
			Enabled:  true,
			Required: false,
			Ready:    true,
		}
	}

	message := "redis client is unavailable"
	if redisStatus.Error != nil {
		message = redisStatus.Error.Error()
	}
	return dependencyStatus{
		Enabled:  true,
		Required: false,
		Ready:    false,
		Message:  message,
	}
}

func buildQueueReadiness(cfg *config.Conf) dependencyStatus {
	if cfg == nil || !cfg.Queue.Enable {
		return dependencyStatus{
			Enabled:  false,
			Required: false,
			Ready:    false,
			Message:  "queue is disabled",
		}
	}

	if queue.PublisherOrNil() != nil {
		return dependencyStatus{
			Enabled:  true,
			Required: false,
			Ready:    true,
		}
	}

	message := "queue publisher is unavailable"
	if err := queue.PublisherInitError(); err != nil {
		message = err.Error()
	}
	return dependencyStatus{
		Enabled:  true,
		Required: false,
		Ready:    false,
		Message:  message,
	}
}
