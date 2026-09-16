package routers

import (
	"fmt"
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
	return SetRoutersWithTree(AppRouteTreeWithOptions(currentBuildOptions()))
}

// SetRoutersWithTree 使用指定路由树创建 Gin 引擎并注册路由。
func SetRoutersWithTree(routeTree RouteGroupDef) (*gin.Engine, error) {
	catalog, err := CompileRouteTree(routeTree)
	if err != nil {
		return nil, fmt.Errorf("route tree validation failed: %w", err)
	}
	return SetRoutersWithCatalog(catalog)
}

// SetRoutersWithCatalog 使用已编译 Catalog 创建 Gin 引擎，供需要复用元数据的调用方使用。
func SetRoutersWithCatalog(catalog *Catalog) (*gin.Engine, error) {
	if catalog == nil {
		return nil, fmt.Errorf("route catalog is nil")
	}
	engine, err := createEngine()
	if err != nil {
		return nil, err
	}
	RegisterCatalog(engine, catalog)

	// 统一处理 404
	engine.NoRoute(func(c *gin.Context) {
		response2.Resp().SetHttpCode(http.StatusNotFound).FailCode(c, errors.NotFound)
	})

	return engine, nil
}

// createEngine 创建 gin 引擎并设置相关中间件
func createEngine() (*gin.Engine, error) {
	cfg := config.GetConfig()
	globalRateLimit := buildGlobalRateLimitConfig(cfg)

	// 启动时校验 CORS 配置
	middleware.ValidateCORSConfig(cfg)

	debug := cfg != nil && cfg.Debug
	var engine *gin.Engine
	if debug {
		gin.SetMode(gin.DebugMode)
		engine = gin.New()
	} else {
		engine = ReleaseRouter()
	}
	middlewares := []gin.HandlerFunc{
		middleware.CorsHandler(),
		middleware.SecurityHeadersHandler(),
		middleware.RateLimitHandler(globalRateLimit),
		loginRateLimitHandler(),
		middleware.RequestCostHandler(),
		middleware.RequestLocaleHandler(),
		middleware.RequestBodyLimitHandler(),
		middleware.ParseTokenHandler(),
	}
	if debug {
		middlewares = append(middlewares, gin.Logger())
	}
	middlewares = append(middlewares, middleware.CustomRecovery(), middleware.CustomLogger())
	engine.Use(middlewares...)
	// 配置受信任代理，决定是否信任 X-Forwarded-For / X-Real-IP 等代理头。
	trustedProxies := []string(nil)
	if cfg != nil {
		trustedProxies = cfg.TrustedProxies
	}
	if err := engine.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies failed: %w", err)
	}

	return engine, nil
}

func buildGlobalRateLimitConfig(cfg *config.Conf) middleware.RateLimitConfig {
	if cfg == nil {
		return middleware.RateLimitConfig{Enabled: false}
	}
	return middleware.RateLimitConfig{
		Enabled: cfg.RateLimitEnabled,
		RPS:     cfg.RateLimitRPS,
		Burst:   cfg.RateLimitBurst,
		Name:    "global",
	}
}

func buildLoginRateLimitConfig() middleware.RateLimitConfig {
	cfg := config.GetConfig()
	if cfg == nil {
		return middleware.RateLimitConfig{Enabled: false}
	}
	return middleware.RateLimitConfig{
		Enabled: cfg.LoginRateLimitEnabled,
		RPS:     cfg.LoginRateLimitRPS,
		Burst:   cfg.LoginRateLimitBurst,
		Name:    "login",
	}
}

func loginRateLimitHandler() gin.HandlerFunc {
	handler := middleware.RateLimitHandler(buildLoginRateLimitConfig())
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost || c.Request.URL.Path != "/admin/v1/login" {
			c.Next()
			return
		}
		handler(c)
	}
}

// ReleaseRouter 生产模式使用官方建议设置为 release 模式
func ReleaseRouter() *gin.Engine {
	// 切换到生产模式
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()

	return engine
}

func currentBuildOptions() BuildOptions {
	cfg := config.GetConfig()
	return BuildOptions{Debug: cfg != nil && cfg.Debug}
}

// AppRouteTree 返回应用完整路由树。
func AppRouteTree() RouteGroupDef {
	return AppRouteTreeWithOptions(currentBuildOptions())
}

// AppRouteTreeWithOptions 使用显式构建参数返回应用完整路由树。
func AppRouteTreeWithOptions(options BuildOptions) RouteGroupDef {
	return RouteGroupDef{
		Security: SecurityPublic,
		Routes: []RouteDef{
			{
				Method: http.MethodGet,
				Path:   "ping",
				Title:  "ping",
				Desc:   "服务心跳检测接口",
				Handlers: []gin.HandlerFunc{func(c *gin.Context) {
					c.String(http.StatusOK, "pong")
				}},
			},
			{
				Method: http.MethodGet,
				Path:   "health/readiness",
				Title:  "readiness",
				Desc:   "服务依赖就绪状态",
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
		Children: []RouteGroupDef{AdminRouteTreeWithOptions(nil, options)},
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
	if mysqlStatus.Error == nil {
		message = "ok"
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
	if redisStatus.Error == nil {
		message = "ok"
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
	if err := queue.PublisherInitError(); err == nil {
		message = "ok"
	}
	return dependencyStatus{
		Enabled:  true,
		Required: false,
		Ready:    false,
		Message:  message,
	}
}
