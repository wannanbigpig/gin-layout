package routers

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 根据路由树递归注册 Gin 路由。
func RegisterRoutes(engine *gin.Engine, root RouteGroupDef) {
	registerGroup(&engine.RouterGroup, root)
}

func registerGroup(routes *gin.RouterGroup, group RouteGroupDef) {
	current := routes
	if group.Prefix != "" || len(group.Middleware) > 0 {
		current = routes.Group(normalizeRelativePath(group.Prefix), group.Middleware...)
	}

	for _, route := range group.Routes {
		current.Handle(route.Method, normalizeRelativePath(route.Path), route.Handlers...)
	}

	for _, child := range group.Children {
		registerGroup(current, child)
	}
}

func normalizeRelativePath(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return ""
	}
	return "/" + trimmed
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
