package routers

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	response2 "github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

func SetRouters(initApiTable bool) (*gin.Engine, ApiMap) {
	engine := createEngine()

	register := &RegisterRouter{
		InitApiTable: initApiTable,
		Engine:       engine,
	}

	// ping
	register.route(engine, http.MethodGet, "/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	}).setTitle("ping").setAuth(0).setDesc("服务心跳检测接口")

	// 设置 API 路由
	SetAdminApiRoute(register)

	// 统一处理 404
	engine.NoRoute(func(c *gin.Context) {
		response2.Resp().SetHttpCode(http.StatusNotFound).FailCode(c, errors.NotFound)
	})

	if initApiTable {
		return engine, apiMap
	}

	return engine, nil
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
	// set up trusted agents
	if err := engine.SetTrustedProxies([]string{"127.0.0.1"}); err != nil {
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

type ApiMap map[string]*api

var apiMap = make(ApiMap)

// RegisterRouter 注册路由
type RegisterRouter struct {
	ApiMap       ApiMap
	Engine       *gin.Engine
	InitApiTable bool
}

// route 注册路由信息
func (r *RegisterRouter) route(e gin.IRoutes, method string, path string, handler ...gin.HandlerFunc) *api {
	api := newApi()
	if r.InitApiTable {
		api.Method = method
		api.Path = path
		api.Auth = 1 // 默认需要鉴权
		api.GroupCode = ""
		code := utils.MD5(method + "_" + api.Path)
		// 初始化 api 信息
		apiMap[code] = api
	}

	e.Handle(method, path, handler...)
	return api
}

func (r *RegisterRouter) group(relativePath string, handler ...gin.HandlerFunc) *GroupHandler {
	return &GroupHandler{RouterGroup: r.Engine.Group(relativePath, handler...), initApiTable: r.InitApiTable}
}

type GroupHandler struct {
	*gin.RouterGroup
	initApiTable bool
	GroupCode    string
}

// setGroupCode 设置分组code（用于api权限管理，分组code取之于a_api_group表的code字段,需要提前在a_api_group表中添加对应的分组）
func (g *GroupHandler) setGroupCode(code string) *GroupHandler {
	g.GroupCode = code
	return g
}

func (g *GroupHandler) group(relativePath string, handler ...gin.HandlerFunc) *GroupHandler {
	// 创建新的分组，默认继承父分组的 GroupCode
	// 如果子分组需要不同的 GroupCode，可以显式调用 setGroupCode() 来覆盖
	groupHandLer := &GroupHandler{
		RouterGroup:  g.RouterGroup.Group(relativePath, handler...),
		initApiTable: g.initApiTable,
		GroupCode:    g.GroupCode, // 继承父分组的 GroupCode
	}
	return groupHandLer
}

// registerRoute 通用的路由注册函数
func (g *GroupHandler) registerRoute(method string, relativePath string, handler ...gin.HandlerFunc) *api {
	api := newApi()
	if g.initApiTable {
		api.Method = method
		api.Path = strings.TrimSuffix(g.BasePath(), "/") + "/" + strings.Trim(relativePath, "/")
		api.Auth = 1 // 默认需要鉴
		api.GroupCode = g.GroupCode
		code := utils.MD5(method + "_" + api.Path)
		// 初始化 api 信息
		apiMap[code] = api
	}

	g.Handle(method, relativePath, handler...)
	return api
}

// post 注册 POST 路由
func (g *GroupHandler) post(relativePath string, handler ...gin.HandlerFunc) *api {
	return g.registerRoute(http.MethodPost, relativePath, handler...)
}

// get 注册 GET 路由
func (g *GroupHandler) get(relativePath string, handler ...gin.HandlerFunc) *api {
	return g.registerRoute(http.MethodGet, relativePath, handler...)
}

// api 结构体表示route
type api struct {
	Title     string
	Desc      string
	Method    string
	Path      string
	Auth      uint8
	GroupCode string
}

func newApi() *api {
	return &api{}
}

func (r *api) setTitle(title string) *api {
	r.Title = title
	return r
}

func (r *api) setDesc(desc string) *api {
	r.Desc = desc
	return r
}

func (r *api) setAuth(auth uint8) *api {
	r.Auth = auth
	return r
}
