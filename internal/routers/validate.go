package routers

import (
	"fmt"
	"net/http"
	"strings"
)

// RouteTreeError 路由树校验错误。
type RouteTreeError struct {
	Path    string
	Message string
}

func (e *RouteTreeError) Error() string {
	return fmt.Sprintf("route tree error at %s: %s", e.Path, e.Message)
}

// ValidateRouteTree 校验路由树的完整性（启动时调用）。
// 检查项：
// 1. 路由路径非空且合法
// 2. HTTP 方法合法
// 3. Handler 非空
// 4. 重复路由检测
func ValidateRouteTree(root RouteGroupDef) error {
	return validateRouteTree(root, "", make(map[string]bool))
}

func validateRouteTree(group RouteGroupDef, basePath string, seen map[string]bool) error {
	fullPrefix := joinFullPath(basePath, group.Prefix)

	for _, route := range group.Routes {
		// 检查路径非空
		if strings.TrimSpace(route.Path) == "" {
			return &RouteTreeError{Path: fullPrefix, Message: "route path is empty"}
		}

		// 检查 HTTP 方法合法
		if !isValidHTTPMethod(route.Method) {
			return &RouteTreeError{
				Path:    joinFullPath(fullPrefix, route.Path),
				Message: fmt.Sprintf("invalid HTTP method: %s", route.Method),
			}
		}

		// 检查 Handler 非空
		if len(route.Handlers) == 0 {
			return &RouteTreeError{
				Path:    joinFullPath(fullPrefix, route.Path),
				Message: "no handlers registered",
			}
		}

		// 检查重复路由
		routeKey := route.Method + ":" + joinFullPath(fullPrefix, route.Path)
		if seen[routeKey] {
			return &RouteTreeError{
				Path:    routeKey,
				Message: "duplicate route definition",
			}
		}
		seen[routeKey] = true
	}

	for _, child := range group.Children {
		if err := validateRouteTree(child, fullPrefix, seen); err != nil {
			return err
		}
	}

	return nil
}

func isValidHTTPMethod(method string) bool {
	validMethods := map[string]bool{
		http.MethodGet:     true,
		http.MethodPost:    true,
		http.MethodPut:     true,
		http.MethodDelete:  true,
		http.MethodPatch:   true,
		http.MethodHead:    true,
		http.MethodOptions: true,
	}
	return validMethods[method]
}
