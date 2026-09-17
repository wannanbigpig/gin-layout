package routers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

// AuthMode 是持久化到 api.is_auth 的兼容投影。
type AuthMode = global.ApiAuthMode

const (
	AuthModeNone  = global.ApiAuthModeNone
	AuthModeLogin = global.ApiAuthModeLogin
	AuthModeAuth  = global.ApiAuthModeAuth
)

// SecurityPolicy 描述端点实际使用的认证凭证与 API 权限策略。
// Inherit 仅用于子组继承；可注册端点必须最终解析为其他策略。
type SecurityPolicy uint8

const (
	SecurityInherit SecurityPolicy = iota
	SecurityPublic
	SecurityRefreshCookie
	SecurityWebSocketTicket
	SecurityAccessToken
	SecurityAccessTokenPermission
)

func (p SecurityPolicy) String() string {
	switch p {
	case SecurityPublic:
		return "public"
	case SecurityRefreshCookie:
		return "refresh_cookie"
	case SecurityWebSocketTicket:
		return "websocket_ticket"
	case SecurityAccessToken:
		return "access_token"
	case SecurityAccessTokenPermission:
		return "access_token_permission"
	default:
		return "inherit"
	}
}

// AuthMode 返回兼容 api.is_auth 的 0/1/2 投影。
func (p SecurityPolicy) AuthMode() AuthMode {
	switch p {
	case SecurityAccessToken:
		return AuthModeLogin
	case SecurityAccessTokenPermission:
		return AuthModeAuth
	default:
		return AuthModeNone
	}
}

// RouteDef 定义单条路由。认证策略由所属 RouteGroupDef 继承。
type RouteDef struct {
	Method   string
	Path     string
	Title    string
	Desc     string
	Handlers []gin.HandlerFunc
}

func (r RouteDef) WithDesc(desc string) RouteDef {
	r.Desc = desc
	return r
}

// RouteGroupDef 定义共享前缀、认证策略、中间件和分组编码的路由组。
type RouteGroupDef struct {
	Prefix     string
	GroupCode  string
	Security   SecurityPolicy
	Middleware []gin.HandlerFunc
	Routes     []RouteDef
	Children   []RouteGroupDef
}

// RouteMeta 是写入 API 权限表所需的兼容元数据。
type RouteMeta struct {
	Method    string
	Path      string
	Title     string
	Desc      string
	Auth      AuthMode
	Security  SecurityPolicy
	GroupCode string
}

type RouteMetaMap map[string]*RouteMeta

// CompiledRoute 是 Catalog 中已解析完整路径、继承策略和处理器链的端点。
type CompiledRoute struct {
	RouteMeta
	Handlers []gin.HandlerFunc
}

// Catalog 是路由声明树的唯一编译结果，供 Gin、API 同步、OpenAPI 和测试复用。
type Catalog struct {
	routes []CompiledRoute
	meta   RouteMetaMap
}

func (c *Catalog) Routes() []CompiledRoute {
	if c == nil {
		return nil
	}
	routes := make([]CompiledRoute, len(c.routes))
	for i, route := range c.routes {
		routes[i] = route
		routes[i].Handlers = append([]gin.HandlerFunc(nil), route.Handlers...)
	}
	return routes
}

func (c *Catalog) MetaMap() RouteMetaMap {
	if c == nil {
		return nil
	}
	meta := make(RouteMetaMap, len(c.meta))
	for key, route := range c.meta {
		routeCopy := *route
		meta[key] = &routeCopy
	}
	return meta
}

// RouteTreeError 路由树编译错误。
type RouteTreeError struct {
	Path    string
	Message string
}

func (e *RouteTreeError) Error() string {
	return fmt.Sprintf("route tree error at %s: %s", e.Path, e.Message)
}

// CompileRouteTree 一次性完成路径、分组、安全策略和处理器链解析。
func CompileRouteTree(root RouteGroupDef) (*Catalog, error) {
	catalog := &Catalog{meta: make(RouteMetaMap)}
	seen := make(map[string]struct{})
	if err := compileRouteGroup(catalog, root, "", "", SecurityInherit, nil, seen); err != nil {
		return nil, err
	}
	return catalog, nil
}

func compileRouteGroup(catalog *Catalog, group RouteGroupDef, basePath, inheritedGroupCode string, inheritedSecurity SecurityPolicy, inheritedHandlers []gin.HandlerFunc, seen map[string]struct{}) error {
	fullPrefix := joinFullPath(basePath, group.Prefix)
	groupCode := inheritedGroupCode
	if group.GroupCode != "" {
		groupCode = group.GroupCode
	}
	security := inheritedSecurity
	securityChanged := false
	if group.Security != SecurityInherit {
		if !isValidSecurityPolicy(group.Security) {
			return &RouteTreeError{Path: fullPrefix, Message: fmt.Sprintf("invalid security policy: %d", group.Security)}
		}
		securityChanged = group.Security != inheritedSecurity
		if securityChanged && inheritedSecurity != SecurityInherit && inheritedSecurity != SecurityPublic {
			return &RouteTreeError{Path: fullPrefix, Message: fmt.Sprintf("nested security policy transition from %s to %s is not supported", inheritedSecurity, group.Security)}
		}
		security = group.Security
	}

	handlers := append([]gin.HandlerFunc(nil), inheritedHandlers...)
	handlers = append(handlers, group.Middleware...)
	if securityChanged {
		handlers = append(handlers, securityMiddleware(group.Security)...)
	}

	for _, route := range group.Routes {
		if strings.TrimSpace(route.Path) == "" {
			return &RouteTreeError{Path: fullPrefix, Message: "route path is empty"}
		}
		fullPath := joinFullPath(fullPrefix, route.Path)
		if !isValidHTTPMethod(route.Method) {
			return &RouteTreeError{Path: fullPath, Message: fmt.Sprintf("invalid HTTP method: %s", route.Method)}
		}
		if len(route.Handlers) == 0 {
			return &RouteTreeError{Path: fullPath, Message: "no handlers registered"}
		}
		if security == SecurityInherit {
			return &RouteTreeError{Path: fullPath, Message: "security policy is not resolved"}
		}
		key := route.Method + ":" + fullPath
		if _, ok := seen[key]; ok {
			return &RouteTreeError{Path: key, Message: "duplicate route definition"}
		}
		seen[key] = struct{}{}

		meta := RouteMeta{
			Method: route.Method, Path: fullPath, Title: route.Title, Desc: route.Desc,
			Auth: security.AuthMode(), Security: security, GroupCode: groupCode,
		}
		compiled := CompiledRoute{RouteMeta: meta, Handlers: append(append([]gin.HandlerFunc(nil), handlers...), route.Handlers...)}
		catalog.routes = append(catalog.routes, compiled)
		metaCopy := meta
		catalog.meta[utils.MD5(meta.Method+"_"+meta.Path)] = &metaCopy
	}

	for _, child := range group.Children {
		if err := compileRouteGroup(catalog, child, fullPrefix, groupCode, security, handlers, seen); err != nil {
			return err
		}
	}
	return nil
}

func isValidSecurityPolicy(policy SecurityPolicy) bool {
	return policy >= SecurityPublic && policy <= SecurityAccessTokenPermission
}

func securityMiddleware(policy SecurityPolicy) []gin.HandlerFunc {
	switch policy {
	case SecurityAccessToken:
		return []gin.HandlerFunc{middleware.OptionalDatabaseReadyGuard(), middleware.AdminLoginHandler()}
	case SecurityAccessTokenPermission:
		return []gin.HandlerFunc{middleware.OptionalDatabaseReadyGuard(), middleware.AdminLoginHandler(), middleware.AdminPermissionHandler()}
	default:
		return nil
	}
}

func isValidHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// RegisterCatalog 将编译后的端点直接注册到 Gin，避免再次递归解释路由树。
func RegisterCatalog(engine *gin.Engine, catalog *Catalog) {
	for _, route := range catalog.routes {
		handlers := make([]gin.HandlerFunc, 0, len(route.Handlers)+1)
		operationName := route.Title
		handlers = append(handlers, func(c *gin.Context) {
			c.Set(global.ContextKeyOperationName, operationName)
		})
		handlers = append(handlers, route.Handlers...)
		engine.Handle(route.Method, route.Path, handlers...)
	}
}

// RegisterRoutes 保留旧调用入口，内部统一走 Catalog。
func RegisterRoutes(engine *gin.Engine, root RouteGroupDef) {
	catalog, err := CompileRouteTree(root)
	if err != nil {
		panic(err)
	}
	RegisterCatalog(engine, catalog)
}

func ValidateRouteTree(root RouteGroupDef) error {
	_, err := CompileRouteTree(root)
	return err
}

func CollectRouteMeta(root RouteGroupDef) RouteMetaMap {
	catalog, err := CompileRouteTree(root)
	if err != nil {
		return RouteMetaMap{}
	}
	return catalog.MetaMap()
}

func joinFullPath(parts ...string) string {
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.Trim(part, "/")
		if trimmed != "" {
			segments = append(segments, trimmed)
		}
	}
	if len(segments) == 0 {
		return "/"
	}
	return "/" + strings.Join(segments, "/")
}
