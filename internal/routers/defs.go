package routers

import "github.com/gin-gonic/gin"

// RouteDef 定义单条路由。
type RouteDef struct {
	Method   string
	Path     string
	Title    string
	Desc     string
	Auth     uint8
	Handlers []gin.HandlerFunc
}

// RouteGroupDef 定义一组共享前缀、中间件和分组编码的路由。
type RouteGroupDef struct {
	Prefix     string
	GroupCode  string
	Middleware []gin.HandlerFunc
	Routes     []RouteDef
	Children   []RouteGroupDef
}

// RouteMeta 表示写入 API 权限表所需的路由元数据。
type RouteMeta struct {
	Method    string
	Path      string
	Title     string
	Desc      string
	Auth      uint8
	GroupCode string
}

// RouteMetaMap 按 method+path 的哈希值保存路由元数据。
type RouteMetaMap map[string]*RouteMeta
