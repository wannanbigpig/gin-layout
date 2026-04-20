package routers

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
)

// AuthMode 定义路由认证授权模式。
type AuthMode = global.ApiAuthMode

const (
	// AuthModeNone 无需登录，无需权限校验（如：登录、验证码、公开 API）
	AuthModeNone = global.ApiAuthModeNone
	// AuthModeLogin 需要登录，但无需菜单权限校验（如：获取当前用户信息、退出登录）
	AuthModeLogin = global.ApiAuthModeLogin
	// AuthModeAuthz 需要登录且需要api权限校验（如：增删改查业务数据）
	AuthModeAuthz = global.ApiAuthModeAuthz
)

// RouteDef 定义单条路由。
// 建议使用辅助函数 GET()/POST() 创建，避免手写冗长结构。
type RouteDef struct {
	Method   string            // HTTP 方法：GET, POST, PUT, DELETE 等
	Path     string            // 相对路径，如 "list", ":id"
	Title    string            // 路由标题，用于 API 文档
	Desc     string            // 路由描述，补充 Title 未涵盖的信息
	Auth     AuthMode          // 认证授权模式，使用 AuthModeNone/Login/Authz
	Handlers []gin.HandlerFunc // Gin 处理器链
}

// RouteGroupDef 定义一组共享前缀、中间件和分组编码的路由。
// 用于组织路由树结构，支持嵌套分组。
type RouteGroupDef struct {
	Prefix     string            // 路由前缀，如 "admin/v1", "user"
	GroupCode  string            // 分组编码，用于权限分组和 API 文档归类
	Middleware []gin.HandlerFunc // 组内路由共享的中间件（按顺序执行）
	Routes     []RouteDef        // 直接子路由列表
	Children   []RouteGroupDef   // 嵌套子分组（支持无限层级）
}

// RouteMeta 表示写入 API 权限表所需的路由元数据。
// 由 RouteDef 派生，不包含 Handlers（避免序列化）。
type RouteMeta struct {
	Method    string   // HTTP 方法
	Path      string   // 完整路径（含前缀）
	Title     string   // 路由标题
	Desc      string   // 路由描述
	Auth      AuthMode // 认证授权模式
	GroupCode string   // 所属分组编码
}

// RouteMetaMap 按 method+path 的哈希值保存路由元数据。
// 用于快速查找和权限校验。
type RouteMetaMap map[string]*RouteMeta
